package infrastructure

import (
	"context"
	picker "github.com/enterprise-labs/seismic-event-associator/internal/picker/domain"
	"sort"
	"sync"
	"time"
)

type Repository struct {
	mu    sync.RWMutex
	items map[string]picker.PickWithEvidence
}

func NewRepository() *Repository { return &Repository{items: map[string]picker.PickWithEvidence{}} }
func (r *Repository) Save(ctx context.Context, p picker.PickWithEvidence) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items[p.Pick.ID] = p
	return nil
}
func (r *Repository) Get(ctx context.Context, id string) (picker.PickWithEvidence, error) {
	if err := ctx.Err(); err != nil {
		return picker.PickWithEvidence{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.items[id]
	if !ok {
		return p, picker.ErrPickNotFound
	}
	return p, nil
}
func (r *Repository) List(ctx context.Context, from, to time.Time) ([]picker.PickWithEvidence, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []picker.PickWithEvidence{}
	for _, p := range r.items {
		if (from.IsZero() || !p.Pick.Time.Before(from)) && (to.IsZero() || p.Pick.Time.Before(to)) {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Pick.Time.Before(out[j].Pick.Time) })
	return out, nil
}
func (r *Repository) UpdateStatus(ctx context.Context, id, status string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.items[id]
	if !ok {
		return picker.ErrPickNotFound
	}
	p.Pick.Status = status
	r.items[id] = p
	return nil
}
