package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	assoc "github.com/enterprise-labs/seismic-event-associator/internal/association/domain"
	picker "github.com/enterprise-labs/seismic-event-associator/internal/picker/domain"
	station "github.com/enterprise-labs/seismic-event-associator/internal/station/domain"
	travelapp "github.com/enterprise-labs/seismic-event-associator/internal/traveltime/application"
	travel "github.com/enterprise-labs/seismic-event-associator/internal/traveltime/domain"
	"math"
	"sort"
	"time"
)

type StationSource interface {
	Get(context.Context, string, time.Time) (station.Station, error)
}
type Clock interface{ Now() time.Time }
type Locator struct {
	picks            assoc.PickSource
	stations         StationSource
	model            travel.Model
	events           assoc.Repository
	clock            Clock
	algorithmVersion string
}

func NewLocator(picks assoc.PickSource, stations StationSource, model travel.Model, events assoc.Repository, clock Clock, version string) *Locator {
	return &Locator{picks: picks, stations: stations, model: model, events: events, clock: clock, algorithmVersion: version}
}

type candidate struct {
	point      travel.Point
	origin     time.Time
	associated []assoc.AssociatedPick
	rms        float64
	gap        float64
	score      float64
}

func (l *Locator) Associate(ctx context.Context, request assoc.Request) (assoc.Event, error) {
	if request.To.IsZero() {
		request.To = l.clock.Now()
	}
	if request.From.IsZero() {
		request.From = request.To.Add(-2 * time.Minute)
	}
	if request.MinStations < 3 {
		request.MinStations = 3
	}
	if request.MaxResidual <= 0 {
		request.MaxResidual = 1200 * time.Millisecond
	}
	if err := validateGrid(request.Grid); err != nil {
		return assoc.Event{}, err
	}
	picks, err := l.picks.List(ctx, request.From, request.To)
	if err != nil {
		return assoc.Event{}, fmt.Errorf("load picks: %w", err)
	}
	active := make([]picker.PickWithEvidence, 0, len(picks))
	stations := map[string]station.Station{}
	for _, p := range picks {
		if p.Pick.Status != "active" {
			continue
		}
		s, loadErr := l.stations.Get(ctx, p.Pick.StationID, p.Pick.Time)
		if loadErr != nil {
			continue
		}
		stations[p.Pick.StationID] = s
		active = append(active, p)
	}
	if uniqueStations(active) < request.MinStations {
		return assoc.Event{}, fmt.Errorf("need %d unique stations, have %d", request.MinStations, uniqueStations(active))
	}
	best := candidate{rms: math.Inf(1), score: -math.MaxFloat64}
	nodes := 0
	for lat := request.Grid.MinLatitude; lat <= request.Grid.MaxLatitude+1e-9; lat += request.Grid.HorizontalStep {
		for lon := request.Grid.MinLongitude; lon <= request.Grid.MaxLongitude+1e-9; lon += request.Grid.HorizontalStep {
			for _, depth := range request.Grid.DepthsKM {
				nodes++
				if nodes > request.Grid.MaxNodes {
					return assoc.Event{}, fmt.Errorf("grid exceeds max_nodes %d", request.Grid.MaxNodes)
				}
				point := travel.Point{Latitude: lat, Longitude: lon, DepthKM: depth}
				c, ok := l.evaluate(ctx, point, active, stations, request.MaxResidual)
				if !ok {
					continue
				}
				score := float64(len(c.associated))*10 - c.rms/1000 - c.gap/360
				c.score = score
				if score > best.score || (score == best.score && c.rms < best.rms) {
					best = c
				}
			}
		}
	}
	if len(best.associated) < request.MinStations {
		return assoc.Event{}, fmt.Errorf("no grid node met station/residual constraints")
	}
	now := l.clock.Now()
	event := assoc.Event{ID: eventID(best.origin, best.point), Version: 1, OriginTime: best.origin, Hypocenter: best.point, RMSResidualMS: best.rms, AzimuthalGap: best.gap, Confidence: confidence(best, request.MinStations), Status: assoc.Candidate, Picks: best.associated, TravelModelVersion: l.model.Version(), AlgorithmVersion: l.algorithmVersion, CreatedAt: now, UpdatedAt: now}
	if len(best.associated) >= 4 && best.rms < 600 {
		event.Status = assoc.Confirmed
	}
	if previous, ok, lookupErr := l.events.LatestNear(ctx, event.OriginTime, event.Hypocenter, 20, 3*time.Second); lookupErr == nil && ok {
		event.ID = previous.ID
		event.Version = previous.Version + 1
		event.CreatedAt = previous.CreatedAt
		event.Picks = mergePicks(previous.Picks, event.Picks)
		event.Supersedes = []string{fmt.Sprintf("%s@%d", previous.ID, previous.Version)}
		event.Reason = "incremental pick merge"
	}
	if err = l.events.Save(ctx, event); err != nil {
		return event, fmt.Errorf("save event: %w", err)
	}
	return event, nil
}
func (l *Locator) evaluate(ctx context.Context, point travel.Point, picks []picker.PickWithEvidence, stations map[string]station.Station, maxResidual time.Duration) (candidate, bool) {
	inferred := make([]time.Time, 0, len(picks))
	for _, p := range picks {
		tt, err := l.model.TravelTime(ctx, p.Pick.Phase, point, stations[p.Pick.StationID])
		if err != nil {
			continue
		}
		inferred = append(inferred, p.Pick.Time.Add(-time.Duration(tt*float64(time.Second))))
	}
	if len(inferred) < 3 {
		return candidate{}, false
	}
	sort.Slice(inferred, func(i, j int) bool { return inferred[i].Before(inferred[j]) })
	origin := inferred[len(inferred)/2]
	associated := make([]assoc.AssociatedPick, 0, len(picks))
	var squares float64
	usedStations := map[string]bool{}
	bearings := []float64{}
	for _, p := range picks {
		if usedStations[p.Pick.StationID] {
			continue
		}
		s := stations[p.Pick.StationID]
		tt, err := l.model.TravelTime(ctx, p.Pick.Phase, point, s)
		if err != nil {
			continue
		}
		predicted := origin.Add(time.Duration(tt * float64(time.Second)))
		residual := p.Pick.Time.Sub(predicted).Seconds() * 1000
		if math.Abs(residual) > float64(maxResidual.Milliseconds()) {
			continue
		}
		weight := 1 / math.Max(p.Pick.UncertaintyMS, 1)
		associated = append(associated, assoc.AssociatedPick{PickID: p.Pick.ID, StationID: p.Pick.StationID, Phase: p.Pick.Phase, Observed: p.Pick.Time, Predicted: predicted, ResidualMS: residual, Weight: weight, Amplitude: p.Pick.Amplitude})
		squares += residual * residual
		usedStations[p.Pick.StationID] = true
		bearings = append(bearings, travelapp.BearingDegrees(point.Latitude, point.Longitude, s.Latitude, s.Longitude))
	}
	if len(associated) < 3 {
		return candidate{}, false
	}
	return candidate{point: point, origin: origin, associated: associated, rms: math.Sqrt(squares / float64(len(associated))), gap: azimuthGap(bearings)}, true
}
func validateGrid(g assoc.Grid) error {
	if g.MinLatitude > g.MaxLatitude || g.MinLongitude > g.MaxLongitude {
		return fmt.Errorf("grid minimum exceeds maximum")
	}
	if g.HorizontalStep <= 0 || g.HorizontalStep > 5 {
		return fmt.Errorf("horizontal_step outside bounds")
	}
	if len(g.DepthsKM) == 0 {
		return fmt.Errorf("at least one depth required")
	}
	if g.MaxNodes < 1 || g.MaxNodes > 1_000_000 {
		return fmt.Errorf("max_nodes outside bounds")
	}
	return nil
}
func uniqueStations(picks []picker.PickWithEvidence) int {
	seen := map[string]bool{}
	for _, p := range picks {
		seen[p.Pick.StationID] = true
	}
	return len(seen)
}
func azimuthGap(values []float64) float64 {
	if len(values) < 2 {
		return 360
	}
	sort.Float64s(values)
	largest := 0.0
	for i := range values {
		next := values[(i+1)%len(values)]
		if i == len(values)-1 {
			next += 360
		}
		if gap := next - values[i]; gap > largest {
			largest = gap
		}
	}
	return largest
}
func confidence(c candidate, min int) float64 {
	stationFactor := math.Min(1, float64(len(c.associated))/float64(min+2))
	residualFactor := math.Exp(-c.rms / 1000)
	geometryFactor := 1 - c.gap/360
	value := 0.4*stationFactor + 0.4*residualFactor + 0.2*geometryFactor
	if value < 0 {
		value = 0
	}
	if value > 1 {
		value = 1
	}
	return value
}
func eventID(origin time.Time, p travel.Point) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%d|%.3f|%.3f|%.1f", origin.Unix(), p.Latitude, p.Longitude, p.DepthKM)))
	return "evt_" + hex.EncodeToString(sum[:10])
}
func mergePicks(a, b []assoc.AssociatedPick) []assoc.AssociatedPick {
	seen := map[string]bool{}
	out := make([]assoc.AssociatedPick, 0, len(a)+len(b))
	for _, group := range [][]assoc.AssociatedPick{a, b} {
		for _, p := range group {
			if !seen[p.PickID] {
				seen[p.PickID] = true
				out = append(out, p)
			}
		}
	}
	return out
}
