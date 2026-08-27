package application

import (
	"fmt"
	waveform "github.com/enterprise-labs/seismic-event-associator/internal/waveform/domain"
	"math"
	"sort"
	"time"
)

type Assembler struct {
	RateTolerance    float64
	GapFillLimit     time.Duration
	OverlapTolerance time.Duration
}

func NewAssembler() *Assembler {
	return &Assembler{RateTolerance: 1e-6, GapFillLimit: 100 * time.Millisecond, OverlapTolerance: 20 * time.Millisecond}
}
func (a *Assembler) Assemble(blocks []waveform.SampleBlock) ([]waveform.Stream, error) {
	if len(blocks) == 0 {
		return nil, fmt.Errorf("no waveform blocks")
	}
	items := make([]waveform.SampleBlock, len(blocks))
	copy(items, blocks)
	waveform.SortBlocks(items)
	groups := map[string][]waveform.SampleBlock{}
	order := []string{}
	for _, block := range items {
		if err := block.Validate(); err != nil {
			return nil, err
		}
		key := block.Channel.ID()
		if _, ok := groups[key]; !ok {
			order = append(order, key)
		}
		groups[key] = append(groups[key], block)
	}
	sort.Strings(order)
	streams := make([]waveform.Stream, 0, len(order))
	for _, key := range order {
		parts := groups[key]
		current := waveform.Stream{Channel: parts[0].Channel, Rate: parts[0].Rate, Start: parts[0].Start}
		for _, part := range parts {
			if math.Abs(part.Rate-current.Rate) > a.RateTolerance {
				return nil, fmt.Errorf("sample rate changed for %s", key)
			}
			if current.Blocks == 0 {
				current.Samples = append([]float64(nil), part.Samples...)
				current.Blocks = 1
				continue
			}
			expected := current.End()
			delta := part.Start.Sub(expected)
			sampleDuration := time.Duration(float64(time.Second) / current.Rate)
			if delta > sampleDuration/2 {
				current.Gaps = append(current.Gaps, waveform.TimeRange{Start: expected, End: part.Start})
				if delta <= a.GapFillLimit {
					missing := int(math.Round(delta.Seconds() * current.Rate))
					for i := 0; i < missing; i++ {
						current.Samples = append(current.Samples, math.NaN())
					}
				} else {
					streams = append(streams, current)
					current = waveform.Stream{Channel: part.Channel, Rate: part.Rate, Start: part.Start}
				}
			} else if delta < -a.OverlapTolerance {
				overlap := int(math.Round(-delta.Seconds() * current.Rate))
				if overlap >= len(part.Samples) {
					continue
				}
				part.Samples = part.Samples[overlap:]
			}
			current.Samples = append(current.Samples, part.Samples...)
			current.Blocks++
		}
		if len(current.Samples) > 0 {
			streams = append(streams, current)
		}
	}
	return streams, nil
}
