package datasources

import (
	"context"

	"github.com/PDOK/gomagpie/internal/search/domain"
)

// Datasource knows how make different kinds of queries/actions on the underlying actual datastore.
// This abstraction allows the rest of the system to stay datastore agnostic.
type Datasource interface {
	Suggest(ctx context.Context, searchTerm string, collections map[string]map[string]string,
		srid domain.SRID, limit int) ([]string, error)

	// Close closes (connections to) the datasource gracefully
	Close()
}