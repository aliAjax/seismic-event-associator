package application

import (
	"bytes"
	"context"
	"encoding/binary"
	waveform "github.com/enterprise-labs/seismic-event-associator/internal/waveform/domain"
	"testing"
	"time"
)

func TestEncodeParseFloat32Record(t *testing.T) {
	block := SyntheticBlock(waveform.Channel{Network: "XX", Station: "TEST", Location: "00", Code: "BHZ"}, time.Date(2026, 1, 2, 3, 4, 5, 600000000, time.UTC), 50, 2*time.Second, map[time.Duration]float64{time.Second: 10}, nil)
	block.Sequence = 42
	raw, err := EncodeFloat32Record(block, 512)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := NewParser(SampleDecoder{}, 1<<20).ParseStream(context.Background(), bytes.NewReader(raw), "test")
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.Blocks) != 1 {
		t.Fatalf("blocks=%d", len(parsed.Blocks))
	}
	got := parsed.Blocks[0]
	if got.Channel.ID() != "XX.TEST.00.BHZ" || got.Sequence != 42 || got.Rate != 50 || len(got.Samples) != 100 {
		t.Fatalf("parsed=%+v samples=%d", got, len(got.Samples))
	}
	if got.Start.Sub(block.Start) > 100*time.Microsecond || block.Start.Sub(got.Start) > 100*time.Microsecond {
		t.Fatalf("start mismatch %s %s", got.Start, block.Start)
	}
}
func TestRejectMalformedRecordLength(t *testing.T) {
	block := waveform.SampleBlock{Channel: waveform.Channel{Network: "XX", Station: "TEST", Code: "BHZ"}, Start: time.Now(), Rate: 20, Samples: make([]float64, 20), Sequence: 1}
	raw, err := EncodeFloat32Record(block, 256)
	if err != nil {
		t.Fatal(err)
	}
	raw[54] = 25
	if _, err = NewParser(SampleDecoder{}, 1<<20).ParseStream(context.Background(), bytes.NewReader(raw), "bad"); err == nil {
		t.Fatal("invalid exponent accepted")
	}
	binary.BigEndian.PutUint16(raw[46:48], 250)
	raw[54] = 8
	if _, err = NewParser(SampleDecoder{}, 1<<20).ParseStream(context.Background(), bytes.NewReader(raw), "bad-offset"); err == nil {
		t.Fatal("out-of-bounds blockette accepted")
	}
}
