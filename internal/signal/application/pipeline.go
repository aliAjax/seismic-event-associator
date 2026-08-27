package application

import (
	"fmt"
	signal "github.com/enterprise-labs/seismic-event-associator/internal/signal/domain"
	waveform "github.com/enterprise-labs/seismic-event-associator/internal/waveform/domain"
	"math"
	"time"
)

type Pipeline struct{}

func NewPipeline() *Pipeline { return &Pipeline{} }
func (p *Pipeline) Process(block waveform.SampleBlock, params signal.Parameters) (signal.Trace, error) {
	if err := block.Validate(); err != nil {
		return signal.Trace{}, err
	}
	if err := params.Validate(block.Rate); err != nil {
		return signal.Trace{}, err
	}
	samples := append([]float64(nil), block.Samples...)
	if params.MeanRemove {
		removeMean(samples)
	}
	if params.Detrend {
		detrend(samples)
	}
	if params.FilterLowHz > 0 || params.FilterHighHz > 0 {
		samples = bandPass(samples, block.Rate, params.FilterLowHz, params.FilterHighHz, params.FilterOrder)
	}
	trace := signal.Trace{Channel: block.Channel, Start: block.Start, Rate: block.Rate, Samples: samples, Parameters: params, Gap: block.GapBefore}
	trace.Stats = signal.ComputeStats(samples)
	return trace, nil
}
func removeMean(values []float64) {
	if len(values) == 0 {
		return
	}
	var sum float64
	for _, v := range values {
		sum += v
	}
	mean := sum / float64(len(values))
	for i := range values {
		values[i] -= mean
	}
}
func detrend(values []float64) {
	n := float64(len(values))
	if n < 2 {
		return
	}
	var sx, sy, sxx, sxy float64
	for i, y := range values {
		x := float64(i)
		sx += x
		sy += y
		sxx += x * x
		sxy += x * y
	}
	den := n*sxx - sx*sx
	if den == 0 {
		return
	}
	slope := (n*sxy - sx*sy) / den
	intercept := (sy - slope*sx) / n
	for i := range values {
		values[i] -= intercept + slope*float64(i)
	}
}
func bandPass(values []float64, rate, low, high float64, order int) []float64 {
	if len(values) == 0 {
		return values
	}
	if low <= 0 {
		low = 0.01
	}
	if high >= rate/2 {
		high = rate/2 - 0.01
	}
	lowAlpha := math.Exp(-2 * math.Pi * low / rate)
	highAlpha := math.Exp(-2 * math.Pi * high / rate)
	result := append([]float64(nil), values...)
	for pass := 0; pass < order; pass++ {
		lowFiltered := make([]float64, len(result))
		highFiltered := make([]float64, len(result))
		var lowState, highState float64
		for i, v := range result {
			lowState = lowAlpha*lowState + (1-lowAlpha)*v
			highState = highAlpha*highState + (1-highAlpha)*v
			lowFiltered[i] = lowState
			highFiltered[i] = highState
		}
		for i := range result {
			result[i] = highFiltered[i] - lowFiltered[i]
		}
	}
	return result
}
func STA_LTA(values []float64, rate float64, sta, lta time.Duration) []float64 {
	staN := int(sta.Seconds() * rate)
	ltaN := int(lta.Seconds() * rate)
	if staN < 1 {
		staN = 1
	}
	if ltaN <= staN {
		ltaN = staN + 1
	}
	ratio := make([]float64, len(values))
	var staSum, ltaSum float64
	for i, v := range values {
		power := v * v
		staSum += power
		ltaSum += power
		if i >= staN {
			staSum -= values[i-staN] * values[i-staN]
		}
		if i >= ltaN {
			ltaSum -= values[i-ltaN] * values[i-ltaN]
		}
		if i >= ltaN && ltaSum > 1e-12 {
			ratio[i] = (staSum / float64(staN)) / (ltaSum / float64(ltaN))
		}
	}
	return ratio
}
func Trigger(ratio []float64, trace signal.Trace) []TriggerCandidate {
	p := trace.Parameters
	on, off := p.TriggerOn, p.TriggerOff
	started := -1
	result := make([]TriggerCandidate, 0)
	for i, value := range ratio {
		if started < 0 && value >= on {
			started = i
			continue
		}
		if started >= 0 && (value <= off || i == len(ratio)-1) {
			end := i
			if end-started >= int(p.MinTrigger.Seconds()*trace.Rate) {
				if p.MaxTrigger <= 0 || time.Duration(float64(end-started)/trace.Rate*float64(time.Second)) <= p.MaxTrigger {
					peak := started
					for j := started; j <= end && j < len(ratio); j++ {
						if ratio[j] > ratio[peak] {
							peak = j
						}
					}
					result = append(result, TriggerCandidate{Start: trace.Start.Add(time.Duration(float64(started) / trace.Rate * float64(time.Second))), End: trace.Start.Add(time.Duration(float64(end) / trace.Rate * float64(time.Second))), Peak: trace.Start.Add(time.Duration(float64(peak) / trace.Rate * float64(time.Second))), Ratio: ratio[peak]})
				}
			}
			started = -1
		}
	}
	return result
}

type TriggerCandidate struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
	Peak  time.Time `json:"peak"`
	Ratio float64   `json:"ratio"`
}

func PickPhase(trace signal.Trace, candidate TriggerCandidate, kind string) PhasePick {
	windowStart := candidate.Start
	windowEnd := candidate.End
	if kind == "S" {
		windowStart = candidate.Start.Add(100 * time.Millisecond)
	}
	start := int(windowStart.Sub(trace.Start).Seconds() * trace.Rate)
	end := int(windowEnd.Sub(trace.Start).Seconds() * trace.Rate)
	if start < 0 {
		start = 0
	}
	if end > len(trace.Samples) {
		end = len(trace.Samples)
	}
	if end <= start {
		return PhasePick{Phase: kind, Quality: 0}
	}
	best := start
	bestValue := 0.0
	for i := start; i < end; i++ {
		value := math.Abs(trace.Samples[i])
		if value > bestValue {
			bestValue = value
			best = i
		}
	}
	quality := bestValue / (trace.Stats.RMS + 1e-9)
	if quality > 20 {
		quality = 20
	}
	return PhasePick{Phase: kind, Time: trace.Start.Add(time.Duration(float64(best) / trace.Rate * float64(time.Second))), Amplitude: bestValue, Quality: quality, Method: "abs_peak_after_sta_lta"}
}

type PhasePick struct {
	Phase     string    `json:"phase"`
	Time      time.Time `json:"time"`
	Amplitude float64   `json:"amplitude"`
	Quality   float64   `json:"quality"`
	Method    string    `json:"method"`
}

func ValidateTrace(trace signal.Trace) error {
	if trace.Rate <= 0 || len(trace.Samples) == 0 {
		return fmt.Errorf("trace has no samples")
	}
	if trace.Gap {
		return waveform.ErrGap
	}
	return nil
}
