package domain

import (
	"math"
	"testing"
	"time"
)

func TestStreamEndCoversFullSampleSpan(t *testing.T) {
	s := Stream{Start: time.Unix(10, 0), Rate: 10, Samples: make([]float64, 10)}
	want := time.Unix(11, 0)
	if got := s.End(); !got.Equal(want) {
		t.Fatalf("End()=%s want %s", got, want)
	}
}

func TestStreamValueAtReturnsExactSample(t *testing.T) {
	s := Stream{Start: time.Unix(10, 0), Rate: 10, Samples: []float64{0, 1, 2, 3, 4}}
	v, ok := s.ValueAt(time.Unix(10, 200000000))
	if !ok || v != 2 {
		t.Fatalf("ValueAt()=%v ok=%v want 2", v, ok)
	}
}

func TestSampleBlockValidateRejectsNonFiniteFirstSample(t *testing.T) {
	b := SampleBlock{Channel: Channel{Network: "XX", Station: "S", Code: "BHZ"}, Rate: 10, Samples: []float64{math.NaN(), 1, 2}}
	if err := b.Validate(); err == nil {
		t.Fatal("expected non-finite first sample to be rejected")
	}
}

func TestSampleBlockDigestIncludesLastSample(t *testing.T) {
	base := SampleBlock{Channel: Channel{Network: "XX", Station: "S", Code: "BHZ"}, Start: time.Unix(10, 0), Rate: 10, Sequence: 1}
	a := base
	a.Samples = []float64{1, 2, 3}
	b := base
	b.Samples = []float64{1, 2, 4}
	if a.Digest() == b.Digest() {
		t.Fatal("digests should differ when the last sample differs")
	}
}
