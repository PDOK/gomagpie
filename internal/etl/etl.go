package etl

import (
	"fmt"
	"log"

	"github.com/PDOK/gomagpie/config"
	"github.com/PDOK/gomagpie/internal/etl/extract"
	"github.com/PDOK/gomagpie/internal/etl/load"
	t "github.com/PDOK/gomagpie/internal/etl/transform"
)

// Extract the 'E' of ETL
type Extract interface {
	Extract(featureTable string, fields []string, limit int, offset int) ([]t.RawRecord, error)

	Close() error
}

// Transform the 'T' of ETL
type Transform interface {
	Transform() (t.SearchIndexRecord, error)
}

// Load the 'L' of ETL
type Load interface {
	Load(records []t.RawRecord, fields []string, collection config.GeoSpatialCollection) (int64, error)

	Init() error

	Close() error
}

func CreateSearchIndex(dbConn string) error {
	db, err := load.NewPostgis(dbConn)
	if err != nil {
		return err
	}
	defer db.Close()
	return db.Init()
}

func ImportGeoPackage(cfg *config.Config, gpkgPath string, featureTable string, pageSize int,
	synonymsPath string, substitutionsPath string, targetDbConn string) error {

	log.Printf("start importing GeoPackage %s into Postgres search index", gpkgPath)
	collection, err := getCollectionForTable(cfg, featureTable)
	if err != nil {
		return err
	}
	if collection.Search == nil {
		return fmt.Errorf("no search configuration found for feature table: %s", featureTable)
	}

	source, err := extract.NewGeoPackage(gpkgPath)
	if err != nil {
		return err
	}
	defer source.Close()

	target, err := load.NewPostgis(targetDbConn)
	if err != nil {
		return err
	}
	defer target.Close()

	offset := 0
	for {
		batch, err := source.Extract(featureTable, collection.Search.Fields, pageSize, offset)
		if err != nil {
			return fmt.Errorf("failed reading source: %w", err)
		}
		if len(batch) == 0 {
			break // no more batches of records to load into search index
		}
		loaded, err := target.Load(batch, collection)
		if err != nil {
			return fmt.Errorf("failed importing into target: %w", err)
		}
		log.Printf("imported %d records from GeoPackage into Postgres search index", loaded)
		offset += pageSize
	}
	//
	// query rows (select + rows.next) to slice of structs, with limit+offset
	// transform data
	// copy data to postgres using pgx.copyfromslice
	println(synonymsPath)      // TODO
	println(substitutionsPath) // TODO
	println(targetDbConn)      // TODO

	log.Printf("done importing GeoPackage %s into Postgres search index", gpkgPath)
	return nil
}

func getCollectionForTable(cfg *config.Config, table string) (config.GeoSpatialCollection, error) {
	for _, coll := range cfg.Collections {
		if coll.ID == table {
			return coll, nil
		}
	}
	return config.GeoSpatialCollection{}, fmt.Errorf("no configured collection for feature table: %s", table)
}