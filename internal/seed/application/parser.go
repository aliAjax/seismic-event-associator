package application

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	seed "github.com/enterprise-labs/seismic-event-associator/internal/seed/domain"
	waveform "github.com/enterprise-labs/seismic-event-associator/internal/waveform/domain"
	"io"
	"strconv"
	"strings"
	"time"
)

type Parser struct {
	decoder         seed.Decoder
	maxRecordLength int
}

func NewParser(decoder seed.Decoder, max int) *Parser {
	if max < 256 {
		max = 1 << 20
	}
	return &Parser{decoder: decoder, maxRecordLength: max}
}
func (p *Parser) ParseStream(ctx context.Context, r io.Reader, source string) (waveform.Waveform, error) {
	reader := bufio.NewReaderSize(r, 4096)
	result := waveform.Waveform{Source: source, ReceivedAt: time.Now()}
	seen := map[string]bool{}
	for {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		prefix, err := reader.Peek(256)
		if errors.Is(err, io.EOF) && len(prefix) == 0 {
			break
		}
		if err != nil && len(prefix) < 256 {
			return result, fmt.Errorf("read record prefix: %w", err)
		}
		length, err := recordLength(prefix)
		if err != nil {
			return result, fmt.Errorf("record %d: %w", len(result.Blocks), err)
		}
		if length > p.maxRecordLength {
			return result, fmt.Errorf("record length %d exceeds limit", length)
		}
		raw := make([]byte, length)
		if _, err = io.ReadFull(reader, raw); err != nil {
			return result, fmt.Errorf("read %d-byte record: %w", length, err)
		}
		block, err := p.ParseRecord(raw)
		if err != nil {
			return result, fmt.Errorf("record %d: %w", len(result.Blocks), err)
		}
		digest := block.Digest()
		if seen[digest] {
			return result, fmt.Errorf("%w: digest %s", waveform.ErrDuplicate, digest)
		}
		seen[digest] = true
		block.RecordDigest = digest
		result.Blocks = append(result.Blocks, block)
	}
	waveform.SortBlocks(result.Blocks)
	markGaps(result.Blocks)
	return result, nil
}
func recordLength(prefix []byte) (int, error) {
	if len(prefix) < 64 {
		return 0, io.ErrUnexpectedEOF
	}
	offset := int(binary.BigEndian.Uint16(prefix[46:48]))
	visited := map[int]bool{}
	for steps := 0; steps < 16; steps++ {
		if offset < 48 || offset+4 > len(prefix) {
			return 0, fmt.Errorf("blockette chain offset %d outside prefix", offset)
		}
		if visited[offset] {
			return 0, fmt.Errorf("blockette chain cycle at %d", offset)
		}
		visited[offset] = true
		kind := binary.BigEndian.Uint16(prefix[offset:])
		next := int(binary.BigEndian.Uint16(prefix[offset+2:]))
		if kind == 1000 {
			if offset+8 > len(prefix) {
				return 0, io.ErrUnexpectedEOF
			}
			exp := prefix[offset+6]
			if exp < 8 || exp > 20 {
				return 0, fmt.Errorf("invalid record length exponent %d", exp)
			}
			return 1 << exp, nil
		}
		if next == 0 {
			break
		}
		offset = next
	}
	return 0, fmt.Errorf("blockette 1000 not found in bounded prefix")
}
func (p *Parser) ParseRecord(raw []byte) (waveform.SampleBlock, error) {
	if len(raw) < 64 {
		return waveform.SampleBlock{}, io.ErrUnexpectedEOF
	}
	header, err := parseHeader(raw)
	if err != nil {
		return waveform.SampleBlock{}, err
	}
	if header.RecordLength != len(raw) {
		return waveform.SampleBlock{}, fmt.Errorf("declared record length %d differs from %d", header.RecordLength, len(raw))
	}
	if err = header.Validate(); err != nil {
		return waveform.SampleBlock{}, err
	}
	samples, err := p.decoder.Decode(header.Encoding, header.WordOrder, raw[header.DataOffset:], header.SampleCount)
	if err != nil {
		return waveform.SampleBlock{}, fmt.Errorf("decode samples: %w", err)
	}
	block := waveform.SampleBlock{Channel: header.Channel, Start: header.Start, Rate: header.Rate, Samples: samples, Sequence: header.Sequence, Encoding: encodingName(header.Encoding)}
	return block, block.Validate()
}
func parseHeader(raw []byte) (seed.Header, error) {
	sequenceText := string(raw[0:6])
	sequence, err := strconv.ParseUint(strings.TrimSpace(sequenceText), 10, 64)
	if err != nil {
		return seed.Header{}, fmt.Errorf("invalid sequence %q", sequenceText)
	}
	year := int(binary.BigEndian.Uint16(raw[20:22]))
	day := int(binary.BigEndian.Uint16(raw[22:24]))
	hour, minute, second := int(raw[24]), int(raw[25]), int(raw[26])
	fraction := int(binary.BigEndian.Uint16(raw[28:30]))
	if year < 1900 || year > 2200 || day < 1 || day > 366 || hour > 23 || minute > 59 || second > 60 {
		return seed.Header{}, fmt.Errorf("invalid BTIME fields")
	}
	start := time.Date(year, 1, 1, hour, minute, 0, 0, time.UTC).AddDate(0, 0, day-1).Add(time.Duration(second)*time.Second + time.Duration(fraction)*100*time.Microsecond)
	factor := int16(binary.BigEndian.Uint16(raw[32:34]))
	mult := int16(binary.BigEndian.Uint16(raw[34:36]))
	rate, err := sampleRate(factor, mult)
	if err != nil {
		return seed.Header{}, err
	}
	length, err := recordLength(raw)
	if err != nil {
		return seed.Header{}, err
	}
	b1000 := int(binary.BigEndian.Uint16(raw[46:48]))
	header := seed.Header{Sequence: sequence, Quality: raw[6], Channel: waveform.Channel{Station: strings.TrimSpace(string(raw[8:13])), Location: strings.TrimSpace(string(raw[13:15])), Code: strings.TrimSpace(string(raw[15:18])), Network: strings.TrimSpace(string(raw[18:20]))}, Start: start, SampleCount: int(binary.BigEndian.Uint16(raw[30:32])), Rate: rate, ActivityFlags: raw[36], IOFlags: raw[37], QualityFlags: raw[38], TimeCorrection: int32(binary.BigEndian.Uint32(raw[40:44])), DataOffset: int(binary.BigEndian.Uint16(raw[44:46])), BlocketteOffset: b1000, Encoding: raw[b1000+4], WordOrder: raw[b1000+5], RecordLength: length}
	if header.TimeCorrection != 0 && header.ActivityFlags&0x02 == 0 {
		header.Start = header.Start.Add(time.Duration(header.TimeCorrection) * 100 * time.Microsecond)
	}
	return header, nil
}
func sampleRate(factor, mult int16) (float64, error) {
	if factor == 0 || mult == 0 {
		return 0, fmt.Errorf("zero sample rate factor/multiplier")
	}
	value := float64(1)
	if factor > 0 {
		value *= float64(factor)
	} else {
		value /= float64(-factor)
	}
	if mult > 0 {
		value *= float64(mult)
	} else {
		value /= float64(-mult)
	}
	return value, nil
}
func encodingName(value byte) string {
	switch value {
	case 1:
		return "int16"
	case 3:
		return "int32"
	case 4:
		return "float32"
	case 5:
		return "float64"
	case 10:
		return "steim1"
	case 11:
		return "steim2"
	}
	return "unknown"
}
func markGaps(blocks []waveform.SampleBlock) {
	last := map[string]waveform.SampleBlock{}
	for i := range blocks {
		key := blocks[i].Channel.ID()
		if previous, ok := last[key]; ok {
			tolerance := time.Duration(float64(time.Second) / blocks[i].Rate * 1.5)
			delta := blocks[i].Start.Sub(previous.End())
			if delta > tolerance {
				blocks[i].GapBefore = true
			}
		}
		last[key] = blocks[i]
	}
}
