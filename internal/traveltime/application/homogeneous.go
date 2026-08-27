package application

import (
	"context"
	"fmt"
	station "github.com/enterprise-labs/seismic-event-associator/internal/station/domain"
	travel "github.com/enterprise-labs/seismic-event-associator/internal/traveltime/domain"
	"math"
)

type Homogeneous struct {
	PVelocity    float64
	SVelocity    float64
	modelVersion string
}

func NewHomogeneous(p, s float64, version string) (*Homogeneous, error) {
	if p <= 0 || s <= 0 || p <= s {
		return nil, fmt.Errorf("velocities must satisfy P > S > 0")
	}
	return &Homogeneous{PVelocity: p, SVelocity: s, modelVersion: version}, nil
}
func (m *Homogeneous) Version() string { return m.modelVersion }
func (m *Homogeneous) TravelTime(ctx context.Context, phase string, source travel.Point, target station.Station) (float64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if err := source.Validate(); err != nil {
		return 0, err
	}
	horizontal := GreatCircleKM(source.Latitude, source.Longitude, target.Latitude, target.Longitude)
	vertical := source.DepthKM + target.Elevation/1000
	distance := math.Hypot(horizontal, vertical)
	velocity := m.PVelocity
	if phase == "S" {
		velocity = m.SVelocity
	} else if phase != "P" {
		return 0, fmt.Errorf("unsupported phase %q", phase)
	}
	return distance / velocity, nil
}
func GreatCircleKM(lat1, lon1, lat2, lon2 float64) float64 {
	toRad := math.Pi / 180
	phi1, phi2 := lat1*toRad, lat2*toRad
	dphi := (lat2 - lat1) * toRad
	dlambda := math.Mod((lon2-lon1+540), 360) - 180
	dlambda *= toRad
	a := math.Sin(dphi/2)*math.Sin(dphi/2) + math.Cos(phi1)*math.Cos(phi2)*math.Sin(dlambda/2)*math.Sin(dlambda/2)
	if a > 1 {
		a = 1
	}
	return 6371.0088 * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}
func BearingDegrees(lat1, lon1, lat2, lon2 float64) float64 {
	r := math.Pi / 180
	phi1, phi2 := lat1*r, lat2*r
	dlon := (lon2 - lon1) * r
	y := math.Sin(dlon) * math.Cos(phi2)
	x := math.Cos(phi1)*math.Sin(phi2) - math.Sin(phi1)*math.Cos(phi2)*math.Cos(dlon)
	bearing := math.Atan2(y, x) / r
	if bearing < 0 {
		bearing += 360
	}
	return bearing
}
