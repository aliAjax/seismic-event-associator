package domain

import (
	"context"
	"errors"
	"time"
)

var ErrEvidenceNotFound = errors.New("evidence not found")

type Kind string

const (
	WaveformSummary     Kind = "waveform_summary"
	PickEvidence        Kind = "pick_evidence"
	AssociationEvidence Kind = "association_evidence"
	MagnitudeEvidence   Kind = "magnitude_evidence"
)

type Item struct {
	ID               string         `json:"id"`
	Kind             Kind           `json:"kind"`
	SubjectID        string         `json:"subject_id"`
	AlgorithmVersion string         `json:"algorithm_version"`
	Parameters       map[string]any `json:"parameters"`
	InputDigests     []string       `json:"input_digests"`
	Payload          any            `json:"payload"`
	CreatedAt        time.Time      `json:"created_at"`
}
type Repository interface {
	Save(context.Context, Item) error
	Get(context.Context, string) (Item, error)
	ListBySubject(context.Context, string) ([]Item, error)
}
