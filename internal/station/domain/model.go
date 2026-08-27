package domain

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

var ErrInvalidStation = errors.New("invalid station")
var ErrStationNotFound = errors.New("station not found")

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
func (c Catalog) Get(id string, at time.Time) (Station, bool) {
	s, ok := c.Stations[id]
	if !ok || !s.Enabled || at.Before(s.ValidFrom) || (s.ValidTo != nil && !at.Before(*s.ValidTo)) {
		return Station{}, false
	}
	return s, true
}
