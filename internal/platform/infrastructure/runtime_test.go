package infrastructure

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	platform "github.com/enterprise-labs/seismic-event-associator/internal/platform/application"
	seedapp "github.com/enterprise-labs/seismic-event-associator/internal/seed/application"
	waveform "github.com/enterprise-labs/seismic-event-associator/internal/waveform/domain"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPMiniSEEDAndDuplicate(t *testing.T) {
	ctx := context.Background()
	cfg := platform.DefaultConfig()
	runtime, err := NewRuntime(ctx, cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(runtime.Server.Handler)
	defer server.Close()
	health, err := http.Get(server.URL + "/healthz")
	if err != nil || health.StatusCode != 200 {
		t.Fatalf("health status=%v err=%v", health.StatusCode, err)
	}
	health.Body.Close()
	block := seedapp.SyntheticBlock(waveform.Channel{Network: "XX", Station: "STA1", Location: "00", Code: "BHZ"}, time.Now().UTC(), 50, 12*time.Second, map[time.Duration]float64{8 * time.Second: 30}, nil)
	block.Sequence = 1
	raw, err := seedapp.EncodeFloat32Record(block, 4096)
	if err != nil {
		t.Fatal(err)
	}
	payload := map[string]any{"source": "test", "miniseed_base64": base64.StdEncoding.EncodeToString(raw)}
	body, _ := json.Marshal(payload)
	send := func(data []byte) (int, []byte) {
		req, _ := http.NewRequest(http.MethodPost, server.URL+"/v1/waveforms", bytes.NewReader(data))
		req.Header.Set("X-API-Key", cfg.APIKey)
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		result, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return resp.StatusCode, result
	}
	status, response := send(body)
	if status != 201 {
		t.Fatalf("ingest=%d %s", status, response)
	}
	status, response = send(body)
	if status != http.StatusConflict {
		t.Fatalf("duplicate=%d %s", status, response)
	}
	bad, _ := json.Marshal(map[string]any{"miniseed_base64": "AAE="})
	status, _ = send(bad)
	if status != 400 {
		t.Fatalf("malformed status=%d", status)
	}
}
