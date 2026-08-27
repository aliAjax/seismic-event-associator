package domain

import (
	"context"
	"errors"
	waveform "github.com/enterprise-labs/seismic-event-associator/internal/waveform/domain"
	"time"
)

var ErrPickNotFound = errors.New("pick not found")

type Pick struct {
	ID               string           `json:"id"`
	StationID        string           `json:"station_id"`
	Channel          waveform.Channel `json:"channel"`
	Phase            string           `json:"phase"`
	Time             time.Time        `json:"time"`
	UncertaintyMS    float64          `json:"uncertainty_ms"`
	Amplitude        float64          `json:"amplitude"`
	SNR              float64          `json:"snr"`
	TriggerRatio     float64          `json:"trigger_ratio"`
	AlgorithmVersion string           `json:"algorithm_version"`
	ParameterVersion string           `json:"parameter_version"`
	RecordDigest     string           `json:"record_digest"`
	Status           string           `json:"status"`
	CreatedAt        time.Time        `json:"created_at"`
}
type Evidence struct {
	NoiseMedian    float64   `json:"noise_median"`
	NoiseMAD       float64   `json:"noise_mad"`
	AdaptiveOn     float64   `json:"adaptive_on"`
	AdaptiveOff    float64   `json:"adaptive_off"`
	TriggerStart   time.Time `json:"trigger_start"`
	TriggerEnd     time.Time `json:"trigger_end"`
	PeakSampleTime time.Time `json:"peak_sample_time"`
}
type PickWithEvidence struct {
	Pick     Pick     `json:"pick"`
	Evidence Evidence `json:"evidence"`
}
type Repository interface {
	Save(context.Context, PickWithEvidence) error
	List(context.Context, time.Time, time.Time) ([]PickWithEvidence, error)
	Get(context.Context, string) (PickWithEvidence, error)
	UpdateStatus(context.Context, string, string) error
}
