package domain

import (
	"context"
	"fmt"
	station "github.com/enterprise-labs/seismic-event-associator/internal/station/domain"
)

type Point struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	DepthKM   float64 `json:"depth_km"`
}

func (p Point) Validate() error {
	if p.Latitude < -90 || p.Latitude > 90 || p.Longitude < -180 || p.Longitude > 180 || p.DepthKM < 0 || p.DepthKM > 800 {
		return fmt.Errorf("invalid hypocenter coordinates")
	}
	return nil
}

type Model interface {
	TravelTime(context.Context, string, Point, station.Station) (float64, error)
	Version() string
}
