package domain

import (
	"context"
	"errors"
	picker "github.com/enterprise-labs/seismic-event-associator/internal/picker/domain"
	travel "github.com/enterprise-labs/seismic-event-associator/internal/traveltime/domain"
	"time"
)

var ErrEventNotFound = errors.New("event not found")

type Status string

const (
	Candidate Status = "candidate"
	Confirmed Status = "confirmed"
	Split     Status = "split"
	Merged    Status = "merged"
	Revoked   Status = "revoked"
)

// Active reports whether a status represents a live event that may still be
// operated on by association, merge or split. Terminal statuses (split,
// merged, revoked) must never re-enter the active result set: a split child is
// superseded, a merged event is folded into another, and a revoked event has
// been invalidated.
func (s Status) Active() bool {
	switch s {
	case Candidate, Confirmed:
		return true
	default:
		return false
	}
}

type AssociatedPick struct {
	PickID     string    `json:"pick_id"`
	StationID  string    `json:"station_id"`
	Phase      string    `json:"phase"`
	Observed   time.Time `json:"observed"`
	Predicted  time.Time `json:"predicted"`
	ResidualMS float64   `json:"residual_ms"`
	Weight     float64   `json:"weight"`
	Amplitude  float64   `json:"amplitude"`
}
type Event struct {
	ID                 string           `json:"id"`
	Version            int64            `json:"version"`
	OriginTime         time.Time        `json:"origin_time"`
	Hypocenter         travel.Point     `json:"hypocenter"`
	RMSResidualMS      float64          `json:"rms_residual_ms"`
	AzimuthalGap       float64          `json:"azimuthal_gap"`
	Confidence         float64          `json:"confidence"`
	Status             Status           `json:"status"`
	Picks              []AssociatedPick `json:"picks"`
	Magnitude          *float64         `json:"magnitude,omitempty"`
	TravelModelVersion string           `json:"travel_model_version"`
	AlgorithmVersion   string           `json:"algorithm_version"`
	CreatedAt          time.Time        `json:"created_at"`
	UpdatedAt          time.Time        `json:"updated_at"`
	Supersedes         []string         `json:"supersedes,omitempty"`
	Reason             string           `json:"reason,omitempty"`
}
type Grid struct {
	MinLatitude    float64   `json:"min_latitude"`
	MaxLatitude    float64   `json:"max_latitude"`
	MinLongitude   float64   `json:"min_longitude"`
	MaxLongitude   float64   `json:"max_longitude"`
	HorizontalStep float64   `json:"horizontal_step"`
	DepthsKM       []float64 `json:"depths_km"`
	MaxNodes       int       `json:"max_nodes"`
}
type Request struct {
	From        time.Time     `json:"from"`
	To          time.Time     `json:"to"`
	Grid        Grid          `json:"grid"`
	MinStations int           `json:"min_stations"`
	MaxResidual time.Duration `json:"max_residual"`
}
type Repository interface {
	Save(context.Context, Event) error
	Get(context.Context, string) (Event, error)
	History(context.Context, string) ([]Event, error)
	List(context.Context) ([]Event, error)
	LatestNear(context.Context, time.Time, travel.Point, float64, time.Duration) (Event, bool, error)
}
type PickSource interface {
	List(context.Context, time.Time, time.Time) ([]picker.PickWithEvidence, error)
}
