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
	// order preserves the insertion order of pick IDs so List returns a stable,
	// time-sorted view without relying on map iteration.
	order []string
}

func NewRepository() *Repository {
	return &Repository{items: map[string]picker.PickWithEvidence{}}
}

func (r *Repository) Save(ctx context.Context, p picker.PickWithEvidence) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.items[p.Pick.ID]; !exists {
		r.order = append(r.order, p.Pick.ID)
	}
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

// inWindow reports whether t falls inside the [from, to) half-open window. A
// zero from/to disables the respective bound.
func inWindow(t, from, to time.Time) bool {
	if !from.IsZero() && t.Before(from) {
		return false
	}
	if !to.IsZero() && !t.Before(to) {
		return false
	}
	return true
}

func (r *Repository) List(ctx context.Context, from, to time.Time) ([]picker.PickWithEvidence, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	out := make([]picker.PickWithEvidence, 0, len(r.order))
	for _, id := range r.order {
		p := r.items[id]
		if !inWindow(p.Pick.Time, from, to) {
			continue
		}
		out = append(out, p)
	}
	r.mu.RUnlock()
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
