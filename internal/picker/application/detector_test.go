package application

import (
	"context"
	mem "github.com/enterprise-labs/seismic-event-associator/internal/picker/infrastructure"
	clockpkg "github.com/enterprise-labs/seismic-event-associator/internal/platform/domain"
	signalapp "github.com/enterprise-labs/seismic-event-associator/internal/signal/application"
	signal "github.com/enterprise-labs/seismic-event-associator/internal/signal/domain"
	waveform "github.com/enterprise-labs/seismic-event-associator/internal/waveform/domain"
	"math"
	"testing"
	"time"
)

func TestDetectorFindsSignalNotNoise(t *testing.T) {
	rate := 50.0
	samples := make([]float64, 1000)
	for i := range samples {
		samples[i] = 0.01 * math.Sin(float64(i)*0.31)
	}
	for i := 500; i < 525; i++ {
		samples[i] += 20 * math.Sin(float64(i-500)*0.8)
	}
	params := signal.Parameters{Version: "test", Detrend: true, MeanRemove: true, FilterLowHz: .5, FilterHighHz: 12, FilterOrder: 2, STA: 500 * time.Millisecond, LTA: 5 * time.Second, TriggerOn: 2, TriggerOff: 1.2, MinTrigger: 100 * time.Millisecond, MaxTrigger: 5 * time.Second}
	block := waveform.SampleBlock{Channel: waveform.Channel{Network: "XX", Station: "STA", Code: "BHZ"}, Start: time.Unix(100, 0), Rate: rate, Samples: samples, Sequence: 1}
	trace, err := signalapp.NewPipeline().Process(block, params)
	if err != nil {
		t.Fatal(err)
	}
	repo := mem.NewRepository()
	detector := NewDetector(repo, &clockpkg.ManualClock{Current: time.Unix(200, 0)}, "test-v1")
	picks, err := detector.Detect(context.Background(), trace, block.Digest())
	if err != nil {
		t.Fatal(err)
	}
	if len(picks) == 0 {
		t.Fatal("signal produced no pick")
	}
	if picks[0].Pick.Phase != "P" || picks[0].Pick.SNR <= 1 {
		t.Fatalf("pick=%+v", picks[0])
	}
}
