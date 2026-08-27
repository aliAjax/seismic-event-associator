package domain

import (
	"fmt"
	waveform "github.com/enterprise-labs/seismic-event-associator/internal/waveform/domain"
	"time"
)

type Header struct {
	Sequence        uint64
	Quality         byte
	Channel         waveform.Channel
	Start           time.Time
	SampleCount     int
	Rate            float64
	ActivityFlags   byte
	IOFlags         byte
	QualityFlags    byte
	TimeCorrection  int32
	DataOffset      int
	BlocketteOffset int
	Encoding        byte
	WordOrder       byte
	RecordLength    int
}

func (h Header) Validate() error {
	if h.Sequence == 0 {
		return fmt.Errorf("sequence must be positive")
	}
	if h.SampleCount < 0 || h.SampleCount > 1_000_000 {
		return fmt.Errorf("sample count %d outside bounds", h.SampleCount)
	}
	if h.Rate <= 0 || h.Rate > 100000 {
		return fmt.Errorf("sample rate %.4f outside bounds", h.Rate)
	}
	if h.DataOffset < 48 || h.DataOffset >= h.RecordLength {
		return fmt.Errorf("data offset %d outside record", h.DataOffset)
	}
	if h.BlocketteOffset < 48 || h.BlocketteOffset >= h.RecordLength {
		return fmt.Errorf("blockette offset %d outside record", h.BlocketteOffset)
	}
	return nil
}

type Decoder interface {
	Decode(encoding byte, wordOrder byte, data []byte, count int) ([]float64, error)
}
