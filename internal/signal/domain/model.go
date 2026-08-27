package domain

import (
	"fmt"
	waveform "github.com/enterprise-labs/seismic-event-associator/internal/waveform/domain"
	"math"
	"time"
)

type Parameters struct {
	Version      string        `json:"version"`
	Detrend      bool          `json:"detrend"`
	MeanRemove   bool          `json:"mean_remove"`
	FilterLowHz  float64       `json:"filter_low_hz"`
	FilterHighHz float64       `json:"filter_high_hz"`
	FilterOrder  int           `json:"filter_order"`
	STA          time.Duration `json:"sta"`
	LTA          time.Duration `json:"lta"`
	TriggerOn    float64       `json:"trigger_on"`
	TriggerOff   float64       `json:"trigger_off"`
	MinTrigger   time.Duration `json:"min_trigger"`
	MaxTrigger   time.Duration `json:"max_trigger"`
}

func (p Parameters) Validate(rate float64) error {
	if p.Version == "" {
		return fmt.Errorf("parameter version required")
	}
	if p.FilterLowHz < 0 || p.FilterHighHz <= p.FilterLowHz || p.FilterHighHz >= rate/2 {
		return fmt.Errorf("invalid filter band for rate %.3f", rate)
	}
	if p.FilterOrder < 1 || p.FilterOrder > 8 {
		return fmt.Errorf("filter order outside bounds")
	}
	if p.STA <= 0 || p.LTA <= p.STA {
		return fmt.Errorf("STA/LTA windows invalid")
	}
	if p.TriggerOff <= 0 || p.TriggerOn <= p.TriggerOff {
		return fmt.Errorf("trigger thresholds invalid")
	}
	return nil
}

type Trace struct {
	Channel    waveform.Channel `json:"channel"`
	Start      time.Time        `json:"start"`
	Rate       float64          `json:"rate"`
	Samples    []float64        `json:"-"`
	Parameters Parameters       `json:"parameters"`
	Gap        bool             `json:"gap"`
	Stats      Stats            `json:"stats"`
}
type Stats struct {
	Mean    float64 `json:"mean"`
	RMS     float64 `json:"rms"`
	Minimum float64 `json:"minimum"`
	Maximum float64 `json:"maximum"`
	Samples int     `json:"samples"`
}

func (t Trace) End() time.Time {
	return t.Start.Add(time.Duration(float64(len(t.Samples)) / t.Rate * float64(time.Second)))
}
func ComputeStats(values []float64) Stats {
	if len(values) == 0 {
		return Stats{}
	}
	s := Stats{Minimum: values[0], Maximum: values[0], Samples: len(values)}
	for _, v := range values {
		s.Mean += v
		if v < s.Minimum {
			s.Minimum = v
		}
		if v > s.Maximum {
			s.Maximum = v
		}
	}
	s.Mean /= float64(len(values))
	for _, v := range values {
		s.RMS += v * v
	}
	s.RMS = math.Sqrt(s.RMS / float64(len(values)))
	return s
}
