package domain

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	assoc "github.com/enterprise-labs/seismic-event-associator/internal/association/domain"
	station "github.com/enterprise-labs/seismic-event-associator/internal/station/domain"
)

type Observation struct {
	PickID     string  `json:"pick_id"`
	StationID  string  `json:"station_id"`
	Amplitude  float64 `json:"amplitude"`
	DistanceKM float64 `json:"distance_km"`
	Correction float64 `json:"correction"`
	Magnitude  float64 `json:"magnitude"`
}

type Estimate struct {
	Value        float64       `json:"value"`
	Method       string        `json:"method"`
	Uncertainty  float64       `json:"uncertainty"`
	Observations []Observation `json:"observations"`
	Version      string        `json:"version"`
	EstimatedAt  time.Time     `json:"estimated_at"`
}

type Estimator interface {
	Estimate(context.Context, assoc.Event, map[string]station.Station) (Estimate, error)
}

type LocalMagnitude struct {
	Version      string
	MinAmplitude float64
}

func NewLocalMagnitude(version string) *LocalMagnitude {
	if version == "" {
		version = "ml-v1"
	}
	return &LocalMagnitude{Version: version, MinAmplitude: 1e-9}
}
func (m *LocalMagnitude) Estimate(ctx context.Context, event assoc.Event, stations map[string]station.Station) (Estimate, error) {
	if err := ctx.Err(); err != nil {
		return Estimate{}, err
	}
	if len(event.Picks) == 0 {
		return Estimate{}, fmt.Errorf("event has no picks")
	}
	obs := make([]Observation, 0, len(event.Picks))
	values := make([]float64, 0, len(event.Picks))
	for _, pick := range event.Picks {
		s, ok := stations[pick.StationID]
		if !ok || pick.Phase != "P" || pick.Amplitude < m.MinAmplitude {
			continue
		}
		distance := distanceKM(event, s)
		correction := mathCorrection(distance)
		value := log10(pick.Amplitude) + correction
		obs = append(obs, Observation{PickID: pick.PickID, StationID: pick.StationID, Amplitude: pick.Amplitude, DistanceKM: distance, Correction: correction, Magnitude: value})
		values = append(values, value)
	}
	if len(values) == 0 {
		return Estimate{}, fmt.Errorf("no usable P amplitude observations")
	}
	median := median(values)
	uncertainty := mad(values)
	return Estimate{Value: median, Method: "local_magnitude", Uncertainty: uncertainty, Observations: obs, Version: m.Version, EstimatedAt: time.Now()}, nil
}
func distanceKM(event assoc.Event, s station.Station) float64 {
	lat := event.Hypocenter.Latitude
	lon := event.Hypocenter.Longitude
	return greatCircle(lat, lon, s.Latitude, s.Longitude)
}
func greatCircle(a, b, c, d float64) float64 {
	const r = 0.017453292519943295
	pa, pb := a*r, c*r
	dp := (c - a) * r
	dl := (d - b) * r
	v := sin(dp/2)*sin(dp/2) + cos(pa)*cos(pb)*sin(dl/2)*sin(dl/2)
	if v > 1 {
		v = 1
	}
	return 6371.0088 * 2 * math.Atan2(sqrt(v), sqrt(1-v))
}
func mathCorrection(distance float64) float64 {
	if distance < 1 {
		distance = 1
	}
	return 1.11*log10(distance) + 0.003*distance
}
func log10(v float64) float64         { return math.Log10(v) }
func sin(v float64) float64           { return math.Sin(v) }
func cos(v float64) float64           { return math.Cos(v) }
func sqrt(v float64) float64          { return math.Sqrt(v) }
func median(values []float64) float64 { sort.Float64s(values); return values[len(values)/2] }
func mad(values []float64) float64 {
	m := median(append([]float64(nil), values...))
	d := make([]float64, len(values))
	for i, v := range values {
		d[i] = math.Abs(v - m)
	}
	return median(d) * 1.4826
}
