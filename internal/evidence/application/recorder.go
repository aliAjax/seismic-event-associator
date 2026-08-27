package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	evidence "github.com/enterprise-labs/seismic-event-associator/internal/evidence/domain"
	"time"
)

type Clock interface{ Now() time.Time }
type Recorder struct {
	repo  evidence.Repository
	clock Clock
}

func NewRecorder(repo evidence.Repository, clock Clock) *Recorder {
	return &Recorder{repo: repo, clock: clock}
}
func (r *Recorder) Record(ctx context.Context, kind evidence.Kind, subject, version string, parameters map[string]any, digests []string, payload any) (evidence.Item, error) {
	if subject == "" {
		return evidence.Item{}, fmt.Errorf("evidence subject required")
	}
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%d", kind, subject, r.clock.Now().UnixNano())))
	item := evidence.Item{ID: "evd_" + hex.EncodeToString(sum[:10]), Kind: kind, SubjectID: subject, AlgorithmVersion: version, Parameters: parameters, InputDigests: append([]string(nil), digests...), Payload: payload, CreatedAt: r.clock.Now()}
	if err := r.repo.Save(ctx, item); err != nil {
		return item, fmt.Errorf("save evidence: %w", err)
	}
	return item, nil
}
func (r *Recorder) ForSubject(ctx context.Context, subject string) ([]evidence.Item, error) {
	items, err := r.repo.ListBySubject(ctx, subject)
	if err != nil {
		return nil, fmt.Errorf("list evidence: %w", err)
	}
	return items, nil
}
