package domain

import (
	"context"
	"errors"
	"time"
)

var (
	ErrJobNotFound  = errors.New("job not found")
	ErrJobCancelled = errors.New("job cancelled")
	ErrLeaseHeld    = errors.New("job lease held")
)

type Status string

const (
	Queued    Status = "queued"
	Running   Status = "running"
	Succeeded Status = "succeeded"
	Failed    Status = "failed"
	Cancelled Status = "cancelled"
)

type Job struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`
	Status      Status    `json:"status"`
	Priority    int       `json:"priority"`
	Attempt     int       `json:"attempt"`
	MaxAttempts int       `json:"max_attempts"`
	Owner       string    `json:"owner,omitempty"`
	LeaseUntil  time.Time `json:"lease_until,omitempty"`
	Progress    float64   `json:"progress"`
	InputDigest string    `json:"input_digest"`
	ResultID    string    `json:"result_id,omitempty"`
	Error       string    `json:"error,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
type Repository interface {
	Create(context.Context, Job) error
	Get(context.Context, string) (Job, error)
	Update(context.Context, Job) error
	List(context.Context) ([]Job, error)
	Claim(context.Context, string, time.Duration) (Job, error)
	Cancel(context.Context, string) error
}
