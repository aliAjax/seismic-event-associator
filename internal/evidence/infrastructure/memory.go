package infrastructure

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	evidence "github.com/enterprise-labs/seismic-event-associator/internal/evidence/domain"
	"sync"
)

type Repository struct {
	mu    sync.RWMutex
	items map[string]evidence.Item
}

func NewRepository() *Repository { return &Repository{items: map[string]evidence.Item{}} }
func (r *Repository) Save(ctx context.Context, item evidence.Item) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items[item.ID] = item
	return nil
}
func (r *Repository) Get(ctx context.Context, id string) (evidence.Item, error) {
	if err := ctx.Err(); err != nil {
		return evidence.Item{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	item, ok := r.items[id]
	if !ok {
		return item, evidence.ErrEvidenceNotFound
	}
	return item, nil
}
func (r *Repository) ListBySubject(ctx context.Context, subject string) ([]evidence.Item, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []evidence.Item{}
	for _, item := range r.items {
		if item.SubjectID == subject {
			out = append(out, item)
		}
	}
	return out, nil
}
func DigestPayload(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
