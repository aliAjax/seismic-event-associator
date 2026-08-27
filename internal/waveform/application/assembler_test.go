package application

import (
	waveform "github.com/enterprise-labs/seismic-event-associator/internal/waveform/domain"
	"testing"
	"time"
)

func TestAssemblerMarksGapAndOrders(t *testing.T) {
	start := time.Unix(10, 0)
	channel := waveform.Channel{Network: "XX", Station: "S", Code: "BHZ"}
	later := waveform.SampleBlock{Channel: channel, Start: start.Add(2 * time.Second), Rate: 10, Samples: make([]float64, 10), Sequence: 2}
	first := waveform.SampleBlock{Channel: channel, Start: start, Rate: 10, Samples: make([]float64, 10), Sequence: 1}
	streams, err := NewAssembler().Assemble([]waveform.SampleBlock{later, first})
	if err != nil {
		t.Fatal(err)
	}
	if len(streams) != 2 || len(streams[0].Gaps) != 1 {
		t.Fatalf("streams=%+v", streams)
	}
}
