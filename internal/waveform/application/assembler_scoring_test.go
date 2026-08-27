package application

import (
	waveform "github.com/enterprise-labs/seismic-event-associator/internal/waveform/domain"
	"testing"
	"time"
)

func TestAssemblePreservesInputBlockOrder(t *testing.T) {
	start := time.Unix(100, 0)
	ch := waveform.Channel{Network: "XX", Station: "S", Code: "BHZ"}
	second := waveform.SampleBlock{Channel: ch, Start: start.Add(time.Second), Rate: 100, Samples: []float64{2}, Sequence: 2}
	first := waveform.SampleBlock{Channel: ch, Start: start, Rate: 100, Samples: []float64{1}, Sequence: 1}
	blocks := []waveform.SampleBlock{second, first}
	if _, err := NewAssembler().Assemble(blocks); err != nil {
		t.Fatal(err)
	}
	if blocks[0].Sequence != 2 || blocks[1].Sequence != 1 {
		t.Fatalf("input block order mutated: %+v", blocks)
	}
}

func TestAssembledStreamOwnsSampleSnapshot(t *testing.T) {
	start := time.Unix(100, 0)
	ch := waveform.Channel{Network: "XX", Station: "S", Code: "BHZ"}
	samples := []float64{1, 2, 3}
	block := waveform.SampleBlock{Channel: ch, Start: start, Rate: 10, Samples: samples, Sequence: 1}
	streams, err := NewAssembler().Assemble([]waveform.SampleBlock{block})
	if err != nil {
		t.Fatal(err)
	}
	if len(streams) != 1 {
		t.Fatalf("streams=%d", len(streams))
	}
	samples[0] = 999
	if streams[0].Samples[0] != 1 {
		t.Fatalf("stream aliased input samples: %+v", streams[0].Samples)
	}
}
