package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	seedapp "github.com/enterprise-labs/seismic-event-associator/internal/seed/application"
	station "github.com/enterprise-labs/seismic-event-associator/internal/station/domain"
	travelapp "github.com/enterprise-labs/seismic-event-associator/internal/traveltime/application"
	travel "github.com/enterprise-labs/seismic-event-associator/internal/traveltime/domain"
	waveform "github.com/enterprise-labs/seismic-event-associator/internal/waveform/domain"
	"io"
	"math"
	"net/http"
	"os"
	"time"
)

type client struct {
	base, key string
	http      *http.Client
}

func (c client) post(ctx context.Context, path string, payload any) (map[string]any, error) {
	raw, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+path, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", c.key)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s returned %d: %s", path, resp.StatusCode, body)
	}
	var result map[string]any
	if err = json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return result, nil
}
func main() {
	base := flag.String("url", "http://127.0.0.1:18336", "service URL")
	key := flag.String("api-key", "seismic-dev", "API key")
	flag.Parse()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	c := client{base: *base, key: *key, http: &http.Client{Timeout: 10 * time.Second}}
	origin := time.Now().UTC().Add(8 * time.Second).Truncate(time.Millisecond)
	source := travel.Point{Latitude: 0, Longitude: 0, DepthKM: 5}
	stations := []station.Station{{Network: "XX", Code: "STA1", Latitude: 0, Longitude: -0.5, Enabled: true, Version: 1, ValidFrom: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)}, {Network: "XX", Code: "STA2", Latitude: 0.5, Longitude: 0, Enabled: true, Version: 1, ValidFrom: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)}, {Network: "XX", Code: "STA3", Latitude: 0, Longitude: 0.5, Enabled: true, Version: 1, ValidFrom: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)}, {Network: "XX", Code: "STA4", Latitude: -0.5, Longitude: 0, Enabled: true, Version: 1, ValidFrom: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)}}
	model, _ := travelapp.NewHomogeneous(6, 3.5, "simulator-v1")
	total := 0
	for i, s := range stations {
		travelSeconds, _ := model.TravelTime(ctx, "P", source, s)
		start := origin.Add(-8 * time.Second)
		arrival := time.Duration((8 + travelSeconds) * float64(time.Second))
		noise := func(n int) float64 { return 0.02 * math.Sin(float64(n)*0.17+float64(i)) }
		block := seedapp.SyntheticBlock(waveform.Channel{Network: s.Network, Station: s.Code, Location: "00", Code: "BHZ"}, start, 50, 22*time.Second, map[time.Duration]float64{arrival: 50}, noise)
		block.Sequence = uint64(i + 1)
		raw, err := seedapp.EncodeFloat32Record(block, 8192)
		if err != nil {
			fatal(err)
		}
		request := map[string]any{"source": "simulator", "miniseed_base64": base64.StdEncoding.EncodeToString(raw), "parameters": map[string]any{"version": "sim-v1", "detrend": true, "mean_remove": true, "filter_low_hz": 0.5, "filter_high_hz": 12, "filter_order": 2, "sta": 500000000, "lta": 5000000000, "trigger_on": 2.0, "trigger_off": 1.2, "min_trigger": 100000000, "max_trigger": 10000000000}}
		result, err := c.post(ctx, "/v1/waveforms", request)
		if err != nil {
			fatal(err)
		}
		count, _ := result["count"].(float64)
		total += int(count)
		fmt.Printf("station=%s travel=%.3fs picks=%d\n", s.ID(), travelSeconds, int(count))
	}
	association := map[string]any{"from": origin.Add(-2 * time.Second), "to": origin.Add(20 * time.Second), "grid": map[string]any{"min_latitude": -0.2, "max_latitude": 0.2, "min_longitude": -0.2, "max_longitude": 0.2, "horizontal_step": 0.1, "depths_km": []float64{0, 5, 10}, "max_nodes": 1000}, "min_stations": 4, "max_residual_ms": 1000}
	event, err := c.post(ctx, "/v1/associations", association)
	if err != nil {
		fatal(err)
	}
	encoded, _ := json.Marshal(event)
	fmt.Printf("total_picks=%d event=%s\n", total, encoded)
}
func fatal(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
