package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	picker "github.com/enterprise-labs/seismic-event-associator/internal/picker/domain"
	signalapp "github.com/enterprise-labs/seismic-event-associator/internal/signal/application"
	signal "github.com/enterprise-labs/seismic-event-associator/internal/signal/domain"
	"math"
	"sort"
	"strings"
	"time"
)

type Clock interface{ Now() time.Time }
type Detector struct {
	repo             picker.Repository
	clock            Clock
	algorithmVersion string
	maxPicks         int
}

func NewDetector(repo picker.Repository, clock Clock, version string) *Detector {
	return &Detector{repo: repo, clock: clock, algorithmVersion: version, maxPicks: 100}
}
func (d *Detector) Detect(ctx context.Context, trace signal.Trace, recordDigest string) ([]picker.PickWithEvidence, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := signalapp.ValidateTrace(trace); err != nil {
		return nil, fmt.Errorf("validate trace: %w", err)
	}
	median, mad := robustNoise(trace.Samples)
	adaptive := trace.Parameters
	noiseScale := mad / (median + 1e-9)
	if noiseScale > 1 {
		adaptive.TriggerOn = math.Max(adaptive.TriggerOn, 2.5+noiseScale)
		adaptive.TriggerOff = math.Max(adaptive.TriggerOff, adaptive.TriggerOn*0.45)
	}
	trace.Parameters = adaptive
	ratio := signalapp.STA_LTA(trace.Samples, trace.Rate, adaptive.STA, adaptive.LTA)
	triggers := signalapp.Trigger(ratio, trace)
	if len(triggers) > d.maxPicks {
		triggers = triggers[:d.maxPicks]
	}
	phase := "S"
	code := strings.ToUpper(trace.Channel.Code)
	if strings.HasSuffix(code, "Z") {
		phase = "P"
	}
	result := make([]picker.PickWithEvidence, 0, len(triggers))
	for index, trigger := range triggers {
		candidate := signalapp.PickPhase(trace, trigger, phase)
		if candidate.Quality < 1.2 {
			continue
		}
		id := pickID(trace.Channel.ID(), candidate.Time, phase, index)
		uncertainty := 1000 / trace.Rate * (1 + 1/math.Max(candidate.Quality, 1))
		item := picker.PickWithEvidence{Pick: picker.Pick{ID: id, StationID: trace.Channel.Network + "." + trace.Channel.Station, Channel: trace.Channel, Phase: phase, Time: candidate.Time, UncertaintyMS: uncertainty, Amplitude: candidate.Amplitude, SNR: candidate.Quality, TriggerRatio: trigger.Ratio, AlgorithmVersion: d.algorithmVersion, ParameterVersion: trace.Parameters.Version, RecordDigest: recordDigest, Status: "active", CreatedAt: d.clock.Now()}, Evidence: picker.Evidence{NoiseMedian: median, NoiseMAD: mad, AdaptiveOn: adaptive.TriggerOn, AdaptiveOff: adaptive.TriggerOff, TriggerStart: trigger.Start, TriggerEnd: trigger.End, PeakSampleTime: candidate.Time}}
		if err := d.repo.Save(ctx, item); err != nil {
			return result, fmt.Errorf("save pick: %w", err)
		}
		result = append(result, item)
	}
	return result, nil
}
func robustNoise(values []float64) (float64, float64) {
	if len(values) == 0 {
		return 0, 0
	}
	absolute := make([]float64, len(values))
	for i, v := range values {
		absolute[i] = math.Abs(v)
	}
	sort.Float64s(absolute)
	median := absolute[len(absolute)/2]
	deviations := make([]float64, len(absolute))
	for i, v := range absolute {
		deviations[i] = math.Abs(v - median)
	}
	sort.Float64s(deviations)
	return median, deviations[len(deviations)/2]
}
func pickID(channel string, t time.Time, phase string, index int) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%s|%d", channel, t.UTC().Format(time.RFC3339Nano), phase, index)))
	return "pick_" + hex.EncodeToString(sum[:10])
}
