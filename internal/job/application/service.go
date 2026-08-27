package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	job "github.com/enterprise-labs/seismic-event-associator/internal/job/domain"
	"time"
)

type Clock interface{ Now() time.Time }
type Service struct {
	repo  job.Repository
	clock Clock
	owner string
}

func NewService(repo job.Repository, clock Clock, owner string) *Service {
	return &Service{repo: repo, clock: clock, owner: owner}
}
func (s *Service) Submit(ctx context.Context, typ, digest string, priority, maxAttempts int) (job.Job, error) {
	if typ == "" || digest == "" {
		return job.Job{}, fmt.Errorf("job type and input digest required")
	}
	if maxAttempts < 1 {
		maxAttempts = 3
	}
	now := s.clock.Now()
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%d", typ, digest, now.UnixNano())))
	item := job.Job{ID: "job_" + hex.EncodeToString(sum[:10]), Type: typ, Status: job.Queued, Priority: priority, MaxAttempts: maxAttempts, InputDigest: digest, CreatedAt: now, UpdatedAt: now}
	if err := s.repo.Create(ctx, item); err != nil {
		return item, fmt.Errorf("create job: %w", err)
	}
	return item, nil
}
func (s *Service) Claim(ctx context.Context, ttl time.Duration) (job.Job, error) {
	item, err := s.repo.Claim(ctx, s.owner, ttl)
	if err != nil {
		return item, err
	}
	return item, nil
}
func (s *Service) Complete(ctx context.Context, id, resultID string) (job.Job, error) {
	item, err := s.repo.Get(ctx, id)
	if err != nil {
		return item, err
	}
	if item.Owner != s.owner {
		return item, fmt.Errorf("job owned by %s", item.Owner)
	}
	item.Status = job.Succeeded
	item.ResultID = resultID
	item.Progress = 1
	item.UpdatedAt = s.clock.Now()
	if err = s.repo.Update(ctx, item); err != nil {
		return item, fmt.Errorf("complete job: %w", err)
	}
	return item, nil
}
func (s *Service) Fail(ctx context.Context, id string, cause error) (job.Job, error) {
	item, err := s.repo.Get(ctx, id)
	if err != nil {
		return item, err
	}
	item.Error = cause.Error()
	item.UpdatedAt = s.clock.Now()
	if item.Attempt < item.MaxAttempts {
		item.Status = job.Queued
		item.Owner = ""
		item.LeaseUntil = time.Time{}
		item.Progress = 0
	} else {
		item.Status = job.Failed
	}
	if err = s.repo.Update(ctx, item); err != nil {
		return item, err
	}
	return item, nil
}
func (s *Service) Cancel(ctx context.Context, id string) error { return s.repo.Cancel(ctx, id) }
func (s *Service) List(ctx context.Context) ([]job.Job, error) { return s.repo.List(ctx) }
