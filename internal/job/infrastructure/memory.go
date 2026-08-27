package infrastructure

import (
	"context"
	job "github.com/enterprise-labs/seismic-event-associator/internal/job/domain"
	"sort"
	"sync"
	"time"
)

type Repository struct {
	mu    sync.Mutex
	items map[string]job.Job
}

func NewRepository() *Repository { return &Repository{items: map[string]job.Job{}} }
func (r *Repository) Create(ctx context.Context, item job.Job) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items[item.ID] = item
	return nil
}
func (r *Repository) Get(ctx context.Context, id string) (job.Job, error) {
	if err := ctx.Err(); err != nil {
		return job.Job{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	item, ok := r.items[id]
	if !ok {
		return item, job.ErrJobNotFound
	}
	return item, nil
}
func (r *Repository) Update(ctx context.Context, item job.Job) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.items[item.ID]; !ok {
		return job.ErrJobNotFound
	}
	r.items[item.ID] = item
	return nil
}
func (r *Repository) List(ctx context.Context) ([]job.Job, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]job.Job, 0, len(r.items))
	for _, item := range r.items {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Priority != out[j].Priority {
			return out[i].Priority > out[j].Priority
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}
func (r *Repository) Claim(ctx context.Context, owner string, ttl time.Duration) (job.Job, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	items := make([]job.Job, 0, len(r.items))
	for _, item := range r.items {
		if item.Status == job.Queued || (item.Status == job.Running && now.After(item.LeaseUntil)) {
			items = append(items, item)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Priority > items[j].Priority })
	if len(items) == 0 {
		return job.Job{}, job.ErrJobNotFound
	}
	item := items[0]
	item.Status = job.Running
	item.Owner = owner
	item.Attempt++
	item.LeaseUntil = now.Add(ttl)
	item.UpdatedAt = now
	r.items[item.ID] = item
	return item, nil
}
func (r *Repository) Cancel(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	item, ok := r.items[id]
	if !ok {
		return job.ErrJobNotFound
	}
	item.Status = job.Cancelled
	item.UpdatedAt = time.Now()
	r.items[id] = item
	return nil
}
