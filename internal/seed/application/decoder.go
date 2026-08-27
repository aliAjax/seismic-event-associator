package application

import (
	"encoding/binary"
	"fmt"
	"math"
)

type SteimDecoder interface {
	DecodeSteim(encoding byte, order binary.ByteOrder, data []byte, count int) ([]float64, error)
}
type SampleDecoder struct{ Steim SteimDecoder }

func (d SampleDecoder) Decode(encoding, wordOrder byte, data []byte, count int) ([]float64, error) {
	var order binary.ByteOrder = binary.BigEndian
	if wordOrder == 0 {
		order = binary.LittleEndian
	} else if wordOrder != 1 {
		return nil, fmt.Errorf("unsupported word order %d", wordOrder)
	}
	samples := make([]float64, count)
	switch encoding {
	case 1:
		if len(data) < count*2 {
			return nil, fmt.Errorf("int16 payload truncated: need %d have %d", count*2, len(data))
		}
		for i := range samples {
			samples[i] = float64(int16(order.Uint16(data[i*2:])))
		}
	case 3:
		if len(data) < count*4 {
			return nil, fmt.Errorf("int32 payload truncated")
		}
		for i := range samples {
			samples[i] = float64(int32(order.Uint32(data[i*4:])))
		}
	case 4:
		if len(data) < count*4 {
			return nil, fmt.Errorf("float32 payload truncated")
		}
		for i := range samples {
			samples[i] = float64(math.Float32frombits(order.Uint32(data[i*4:])))
		}
	case 5:
		if len(data) < count*8 {
			return nil, fmt.Errorf("float64 payload truncated")
		}
		for i := range samples {
			samples[i] = math.Float64frombits(order.Uint64(data[i*8:]))
		}
	case 10, 11:
		if d.Steim == nil {
			return nil, fmt.Errorf("Steim encoding %d requires configured decoder", encoding)
		}
		return d.Steim.DecodeSteim(encoding, order, data, count)
	default:
		return nil, fmt.Errorf("unsupported miniSEED encoding %d", encoding)
	}
	return samples, nil
}
