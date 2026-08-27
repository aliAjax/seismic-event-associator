package infrastructure

import (
	"context"
	"fmt"
	assoc "github.com/enterprise-labs/seismic-event-associator/internal/association/domain"
	travelapp "github.com/enterprise-labs/seismic-event-associator/internal/traveltime/application"
	travel "github.com/enterprise-labs/seismic-event-associator/internal/traveltime/domain"
	"sort"
	"sync"
	"time"
)

type Repository struct {
	mu      sync.RWMutex
	events  map[string]assoc.Event
	history map[string][]assoc.Event
}

func NewRepository() *Repository {
	return &Repository{events: map[string]assoc.Event{}, history: map[string][]assoc.Event{}}
}
func (r *Repository) Save(ctx context.Context, e assoc.Event) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	current, exists := r.events[e.ID]
	if exists && e.Version <= current.Version {
		return fmt.Errorf("event version %d is not newer than %d", e.Version, current.Version)
	}
	r.events[e.ID] = e
	r.history[e.ID] = append(r.history[e.ID], e)
	return nil
}
func (r *Repository) History(ctx context.Context, id string) ([]assoc.Event, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	items, ok := r.history[id]
	if !ok {
		return nil, assoc.ErrEventNotFound
	}
	return append([]assoc.Event(nil), items...), nil
}
func (r *Repository) Get(ctx context.Context, id string) (assoc.Event, error) {
	if err := ctx.Err(); err != nil {
		return assoc.Event{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.events[id]
	if !ok {
		return e, assoc.ErrEventNotFound
	}
	return e, nil
}
func (r *Repository) List(ctx context.Context) ([]assoc.Event, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]assoc.Event, 0, len(r.events))
	for _, e := range r.events {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].OriginTime.Before(out[j].OriginTime) })
	return out, nil
}
func (r *Repository) LatestNear(ctx context.Context, origin time.Time, p travel.Point, distance float64, window time.Duration) (assoc.Event, bool, error) {
	items, err := r.List(ctx)
	if err != nil {
		return assoc.Event{}, false, err
	}
	for i := len(items) - 1; i >= 0; i-- {
		e := items[i]
		if e.Status == assoc.Revoked || e.Status == assoc.Merged {
			continue
		}
		if absDuration(e.OriginTime.Sub(origin)) <= window && travelapp.GreatCircleKM(e.Hypocenter.Latitude, e.Hypocenter.Longitude, p.Latitude, p.Longitude) <= distance {
			return e, true, nil
		}
	}
	return assoc.Event{}, false, nil
}
func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}
