package load

import (
	"context"
	"fmt"

	t "github.com/PDOK/gomagpie/internal/etl/transform"
	"github.com/jackc/pgx/v5"
	pgxgeom "github.com/twpayne/pgx-geom"
	"golang.org/x/text/language"
)

type Postgres struct {
	db  *pgx.Conn
	ctx context.Context
}

func NewPostgres(dbConn string) (*Postgres, error) {
	ctx := context.Background()
	db, err := pgx.Connect(ctx, dbConn)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to database: %w", err)
	}
	// add support for Go <-> PostGIS conversions
	if err := pgxgeom.Register(ctx, db); err != nil {
		return nil, err
	}
	return &Postgres{db: db, ctx: ctx}, nil
}

func (p *Postgres) Close() {
	_ = p.db.Close(p.ctx)
}

func (p *Postgres) Load(records []t.SearchIndexRecord, index string) (int64, error) {
	loaded, err := p.db.CopyFrom(
		p.ctx,
		pgx.Identifier{index},
		[]string{"feature_id", "external_fid", "collection_id", "collection_version", "display_name", "suggest", "geometry_type", "bbox"},
		pgx.CopyFromSlice(len(records), func(i int) ([]interface{}, error) {
			r := records[i]
			return []any{r.FeatureID, r.ExternalFid, r.CollectionID, r.CollectionVersion, r.DisplayName, r.Suggest, r.GeometryType, r.Bbox}, nil
		}),
	)
	if err != nil {
		return -1, fmt.Errorf("unable to copy records: %w", err)
	}
	return loaded, nil
}

// Init initialize search index
func (p *Postgres) Init(index string, lang language.Tag) error {
	// since "create type if not exists" isn't supported by Postgres we use a bit
	// of pl/pgsql to avoid creating the geometry_type when it already exists.
	geometryType := `
		do $$ begin
		    create type geometry_type as enum ('POINT', 'MULTIPOINT', 'LINESTRING', 'MULTILINESTRING', 'POLYGON', 'MULTIPOLYGON');
		exception
		    when duplicate_object then null;
		end $$;`
	_, err := p.db.Exec(p.ctx, geometryType)
	if err != nil {
		return fmt.Errorf("error creating geometry type: %w", err)
	}

	searchIndexTable := fmt.Sprintf(`
	create table if not exists %[1]s (
		id 					serial,
		feature_id 			text 					 not null,
		external_fid        text                     null,
		collection_id 		text					 not null,
		collection_version 	int 					 not null,
		display_name 		text					 not null,
		suggest 			text					 not null,
		geometry_type 		geometry_type			 not null,
		bbox 				geometry(polygon, %[2]d) null,
		primary key (id, collection_id, collection_version)
	) -- partition by list(collection_id);`, index, t.WGS84) // TODO partitioning comes later
	_, err = p.db.Exec(p.ctx, searchIndexTable)
	if err != nil {
		return fmt.Errorf("error creating search index table: %w", err)
	}

	fullTextSearchColumn := fmt.Sprintf(`
		alter table %[1]s add column if not exists ts tsvector
	    generated always as (to_tsvector('simple', suggest)) stored;`, index)
	_, err = p.db.Exec(p.ctx, fullTextSearchColumn)
	if err != nil {
		return fmt.Errorf("error creating full-text search column: %w", err)
	}

	ginIndex := fmt.Sprintf(`create index if not exists ts_idx on  %[1]s using gin(ts);`, index)
	_, err = p.db.Exec(p.ctx, ginIndex)
	if err != nil {
		return fmt.Errorf("error creating GIN index: %w", err)
	}

	// create custom collation to correctly handle "numbers in strings" when sorting results
	// see https://www.postgresql.org/docs/12/collation.html#id-1.6.10.4.5.7.5
	collation := fmt.Sprintf(`create collation if not exists custom_numeric (provider = icu, locale = '%s-u-kn-true');`, lang.String())
	_, err = p.db.Exec(p.ctx, collation)
	if err != nil {
		return fmt.Errorf("error creating numeric collation: %w", err)
	}

	return err
}
