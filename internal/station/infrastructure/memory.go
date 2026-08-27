package infrastructure

import (
	"context"
	station "github.com/enterprise-labs/seismic-event-associator/internal/station/domain"
	"sync"
	"time"
)

type Repository struct {
	mu      sync.RWMutex
	catalog station.Catalog
}

func NewRepository() *Repository { return &Repository{catalog: station.NewCatalog()} }
func (r *Repository) Upsert(ctx context.Context, s station.Station) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.catalog.Upsert(s)
}
func (r *Repository) Get(ctx context.Context, id string, at time.Time) (station.Station, error) {
	if err := ctx.Err(); err != nil {
		return station.Station{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := r.catalog.Lookup(id, at)
	if err := result.Err(id); err != nil {
		return station.Station{}, err
	}
	return result.Station, nil
}
func (r *Repository) List(ctx context.Context) ([]station.Station, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]station.Station, 0, len(r.catalog.Stations))
	for _, s := range r.catalog.Stations {
		out = append(out, s)
	}
	return out, nil
}
