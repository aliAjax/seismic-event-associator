package domain

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

var ErrInvalidStation = errors.New("invalid station")

// ErrStationNotFound is the sentinel upper layers branch on to distinguish
// "the station cannot be used for this request" (a data problem) from genuine
// service failures (which should be alerted on). Every unavailable-station
// cause below wraps it, so errors.Is(err, ErrStationNotFound) is true for all
// four cases while the wrapping variant lets callers tell them apart.
var ErrStationNotFound = errors.New("station not found")

var (
	// ErrStationMissing: no catalog entry exists for the requested id.
	ErrStationMissing = fmt.Errorf("%w: station missing", ErrStationNotFound)
	// ErrStationDisabled: the entry exists but is administratively disabled.
	ErrStationDisabled = fmt.Errorf("%w: station disabled", ErrStationNotFound)
	// ErrStationNotYetValid: the entry's ValidFrom is in the future relative to at.
	ErrStationNotYetValid = fmt.Errorf("%w: station not_yet_valid", ErrStationNotFound)
	// ErrStationExpired: the entry's ValidTo is non-nil and at is not before it.
	ErrStationExpired = fmt.Errorf("%w: station expired", ErrStationNotFound)
)

// LookupStatus reports why a station lookup did (or did not) resolve.
type LookupStatus int

const (
	LookupFound LookupStatus = iota
	LookupMissing
	LookupDisabled
	LookupNotYetValid
	LookupExpired
)

// LookupResult is the structured outcome of a catalog lookup at a point in time.
type LookupResult struct {
	Station Station
	Status  LookupStatus
}

// unavailableFor maps a non- LookupFound status to its sentinel cause error, or
// nil for LookupFound. Each cause wraps ErrStationNotFound and carries one of
// the missing/disabled/not_yet_valid/expired markers in its text.
func (s LookupStatus) cause() error {
	switch s {
	case LookupMissing:
		return ErrStationMissing
	case LookupDisabled:
		return ErrStationDisabled
	case LookupNotYetValid:
		return ErrStationNotYetValid
	case LookupExpired:
		return ErrStationExpired
	default:
		return nil
	}
}

// Err turns a non-found LookupResult into an error that wraps the matching
// sentinel (and therefore ErrStationNotFound), including the station id and the
// cause's marker word. Returns nil when the lookup succeeded.
func (r LookupResult) Err(id string) error {
	if r.Status == LookupFound {
		return nil
	}
	return fmt.Errorf("station %s unavailable: %w", id, r.Status.cause())
}

type Station struct {
	Code      string     `json:"code"`
	Network   string     `json:"network"`
	Latitude  float64    `json:"latitude"`
	Longitude float64    `json:"longitude"`
	Elevation float64    `json:"elevation_m"`
	Enabled   bool       `json:"enabled"`
	Version   int64      `json:"version"`
	ValidFrom time.Time  `json:"valid_from"`
	ValidTo   *time.Time `json:"valid_to,omitempty"`
}

func (s Station) ID() string { return strings.TrimSpace(s.Network) + "." + strings.TrimSpace(s.Code) }
func (s Station) Validate() error {
	if s.Code == "" {
		return fmt.Errorf("%w: code required", ErrInvalidStation)
	}
	if s.Latitude < -90 || s.Latitude > 90 || math.IsNaN(s.Latitude) {
		return fmt.Errorf("%w: latitude out of range", ErrInvalidStation)
	}
	if s.Longitude < -180 || s.Longitude > 180 || math.IsNaN(s.Longitude) {
		return fmt.Errorf("%w: longitude out of range", ErrInvalidStation)
	}
	if s.Version < 1 {
		return fmt.Errorf("%w: version must be positive", ErrInvalidStation)
	}
	return nil
}

type Catalog struct {
	Stations  map[string]Station
	UpdatedAt time.Time
}

func NewCatalog() Catalog { return Catalog{Stations: map[string]Station{}} }
func (c *Catalog) Upsert(s Station) error {
	if err := s.Validate(); err != nil {
		return err
	}
	if previous, ok := c.Stations[s.ID()]; ok && s.Version <= previous.Version {
		return fmt.Errorf("station version %d is not newer than %d", s.Version, previous.Version)
	}
	c.Stations[s.ID()] = s
	c.UpdatedAt = time.Now()
	return nil
}
// Lookup resolves a station by id at the given instant, reporting which of the
// five outcomes occurred (found, missing, disabled, not_yet_valid, expired).
// The order of checks matters: a missing entry short-circuits before validity
// is considered, so an absent id is reported as missing even when it would also
// be out of its validity window had it existed.
func (c Catalog) Lookup(id string, at time.Time) LookupResult {
	s, ok := c.Stations[id]
	if !ok {
		return LookupResult{Status: LookupMissing}
	}
	if !s.Enabled {
		return LookupResult{Status: LookupDisabled}
	}
	if at.Before(s.ValidFrom) {
		return LookupResult{Status: LookupNotYetValid}
	}
	if s.ValidTo != nil && !at.Before(*s.ValidTo) {
		return LookupResult{Status: LookupExpired}
	}
	return LookupResult{Station: s, Status: LookupFound}
}

// Get reports whether a station is usable at the given time. It is retained as
// a convenience over Lookup for callers that only need the found/not-found bit.
func (c Catalog) Get(id string, at time.Time) (Station, bool) {
	r := c.Lookup(id, at)
	return r.Station, r.Status == LookupFound
}
