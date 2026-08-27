package application

import (
	"encoding/binary"
	"fmt"
	waveform "github.com/enterprise-labs/seismic-event-associator/internal/waveform/domain"
	"math"
	"strconv"
	"time"
)

func EncodeFloat32Record(block waveform.SampleBlock, recordLength int) ([]byte, error) {
	if recordLength < 256 || recordLength&(recordLength-1) != 0 {
		return nil, fmt.Errorf("record length must be power of two >=256")
	}
	if len(block.Samples)*4+64 > recordLength {
		return nil, fmt.Errorf("%d samples do not fit record", len(block.Samples))
	}
	if block.Rate < 1 || block.Rate > 32767 || math.Trunc(block.Rate) != block.Rate {
		return nil, fmt.Errorf("encoder requires integral rate")
	}
	raw := make([]byte, recordLength)
	sequence := strconv.FormatUint(block.Sequence, 10)
	for len(sequence) < 6 {
		sequence = "0" + sequence
	}
	copy(raw[0:6], sequence[len(sequence)-6:])
	raw[6] = 'D'
	raw[7] = ' '
	copyField(raw[8:13], block.Channel.Station)
	copyField(raw[13:15], block.Channel.Location)
	copyField(raw[15:18], block.Channel.Code)
	copyField(raw[18:20], block.Channel.Network)
	start := block.Start.UTC()
	binary.BigEndian.PutUint16(raw[20:22], uint16(start.Year()))
	binary.BigEndian.PutUint16(raw[22:24], uint16(start.YearDay()))
	raw[24], raw[25], raw[26] = byte(start.Hour()), byte(start.Minute()), byte(start.Second())
	binary.BigEndian.PutUint16(raw[28:30], uint16(start.Nanosecond()/100000))
	binary.BigEndian.PutUint16(raw[30:32], uint16(len(block.Samples)))
	binary.BigEndian.PutUint16(raw[32:34], uint16(int16(block.Rate)))
	binary.BigEndian.PutUint16(raw[34:36], 1)
	raw[39] = 1
	binary.BigEndian.PutUint16(raw[44:46], 64)
	binary.BigEndian.PutUint16(raw[46:48], 48)
	binary.BigEndian.PutUint16(raw[48:50], 1000)
	raw[52] = 4
	raw[53] = 1
	raw[54] = byte(bitsFor(recordLength))
	for i, v := range block.Samples {
		binary.BigEndian.PutUint32(raw[64+i*4:], math.Float32bits(float32(v)))
	}
	return raw, nil
}
func copyField(dst []byte, value string) {
	for i := range dst {
		dst[i] = ' '
	}
	copy(dst, value)
}
func bitsFor(value int) int {
	bits := 0
	for value > 1 {
		value >>= 1
		bits++
	}
	return bits
}
func SyntheticBlock(channel waveform.Channel, start time.Time, rate float64, duration time.Duration, arrivals map[time.Duration]float64, noise func(int) float64) waveform.SampleBlock {
	count := int(duration.Seconds() * rate)
	samples := make([]float64, count)
	for i := range samples {
		if noise != nil {
			samples[i] = noise(i)
		}
	}
	for offset, amplitude := range arrivals {
		center := int(offset.Seconds() * rate)
		width := int(rate / 4)
		if width < 2 {
			width = 2
		}
		for j := -width; j <= width; j++ {
			idx := center + j
			if idx >= 0 && idx < len(samples) {
				x := float64(j) / float64(width)
				samples[idx] += amplitude * (1 - x*x) * math.Sin(float64(j)*0.9)
			}
		}
	}
	return waveform.SampleBlock{Channel: channel, Start: start, Rate: rate, Samples: samples, Sequence: 1, Encoding: "float32"}
}
