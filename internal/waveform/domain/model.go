package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

var (
	ErrInvalidWaveform = errors.New("invalid waveform")
	ErrGap             = errors.New("waveform contains a data gap")
	ErrDuplicate       = errors.New("duplicate waveform record")
)

type Channel struct {
	Network  string `json:"network"`
	Station  string `json:"station"`
	Location string `json:"location"`
	Code     string `json:"code"`
}

func (c Channel) ID() string {
	return strings.Join([]string{c.Network, c.Station, c.Location, c.Code}, ".")
}

type SampleBlock struct {
	Channel      Channel   `json:"channel"`
	Start        time.Time `json:"start"`
	Rate         float64   `json:"rate"`
	Samples      []float64 `json:"-"`
	Sequence     uint64    `json:"sequence"`
	Encoding     string    `json:"encoding"`
	GapBefore    bool      `json:"gap_before"`
	RecordDigest string    `json:"record_digest"`
}

func (b SampleBlock) End() time.Time {
	if b.Rate <= 0 || len(b.Samples) == 0 {
		return b.Start
	}
	return b.Start.Add(time.Duration(float64(len(b.Samples)) / b.Rate * float64(time.Second)))
}
func (b SampleBlock) Validate() error {
	if b.Channel.Station == "" || b.Channel.Code == "" {
		return fmt.Errorf("%w: station and code required", ErrInvalidWaveform)
	}
	if b.Rate <= 0 || b.Rate > 100000 {
		return fmt.Errorf("%w: sampling rate %.3f outside bounds", ErrInvalidWaveform, b.Rate)
	}
	if len(b.Samples) == 0 || len(b.Samples) > 10_000_000 {
		return fmt.Errorf("%w: sample count outside bounds", ErrInvalidWaveform)
	}
	for i, v := range b.Samples {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return fmt.Errorf("%w: sample %d is non-finite", ErrInvalidWaveform, i)
		}
	}
	return nil
}
func (b SampleBlock) Digest() string {
	h := sha256.New()
	fmt.Fprintf(h, "%s|%d|%.9f|%d|", b.Channel.ID(), b.Start.UnixNano(), b.Rate, b.Sequence)
	for _, v := range b.Samples {
		fmt.Fprintf(h, "%.9g,", v)
	}
	return hex.EncodeToString(h.Sum(nil))
}

type Waveform struct {
	Blocks     []SampleBlock `json:"blocks"`
	Source     string        `json:"source"`
	ReceivedAt time.Time     `json:"received_at"`
}

func (w Waveform) Validate() error {
	if len(w.Blocks) == 0 {
		return fmt.Errorf("%w: no blocks", ErrInvalidWaveform)
	}
	for i := range w.Blocks {
		if err := w.Blocks[i].Validate(); err != nil {
			return fmt.Errorf("block %d: %w", i, err)
		}
		if w.Blocks[i].RecordDigest == "" {
			w.Blocks[i].RecordDigest = w.Blocks[i].Digest()
		}
	}
	return nil
}
func SortBlocks(blocks []SampleBlock) {
	sort.SliceStable(blocks, func(i, j int) bool {
		if blocks[i].Channel.ID() != blocks[j].Channel.ID() {
			return blocks[i].Channel.ID() < blocks[j].Channel.ID()
		}
		return blocks[i].Start.Before(blocks[j].Start)
	})
}

type Stream struct {
	Channel Channel
	Rate    float64
	Samples []float64
	Start   time.Time
	Gaps    []TimeRange
	Blocks  int
}
type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

func (s Stream) End() time.Time {
	if s.Rate <= 0 {
		return s.Start
	}
	return s.Start.Add(time.Duration(float64(len(s.Samples)) / s.Rate * float64(time.Second)))
}
func (s Stream) ValueAt(t time.Time) (float64, bool) {
	if t.Before(s.Start) || !t.Before(s.End()) || s.Rate <= 0 {
		return 0, false
	}
	position := t.Sub(s.Start).Seconds() * s.Rate
	idx := int(position)
	if idx < 0 || idx >= len(s.Samples) {
		return 0, false
	}
	return s.Samples[idx], true
}
