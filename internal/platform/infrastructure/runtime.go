package infrastructure

import (
	"context"
	"fmt"
	assocapp "github.com/enterprise-labs/seismic-event-associator/internal/association/application"
	assocmem "github.com/enterprise-labs/seismic-event-associator/internal/association/infrastructure"
	evidenceapp "github.com/enterprise-labs/seismic-event-associator/internal/evidence/application"
	evidencemem "github.com/enterprise-labs/seismic-event-associator/internal/evidence/infrastructure"
	jobapp "github.com/enterprise-labs/seismic-event-associator/internal/job/application"
	jobmem "github.com/enterprise-labs/seismic-event-associator/internal/job/infrastructure"
	magnitudeapp "github.com/enterprise-labs/seismic-event-associator/internal/magnitude/application"
	magnitude "github.com/enterprise-labs/seismic-event-associator/internal/magnitude/domain"
	pickerapp "github.com/enterprise-labs/seismic-event-associator/internal/picker/application"
	pickermem "github.com/enterprise-labs/seismic-event-associator/internal/picker/infrastructure"
	adapter "github.com/enterprise-labs/seismic-event-associator/internal/platform/adapter"
	platform "github.com/enterprise-labs/seismic-event-associator/internal/platform/application"
	clockpkg "github.com/enterprise-labs/seismic-event-associator/internal/platform/domain"
	seedapp "github.com/enterprise-labs/seismic-event-associator/internal/seed/application"
	signalapp "github.com/enterprise-labs/seismic-event-associator/internal/signal/application"
	station "github.com/enterprise-labs/seismic-event-associator/internal/station/domain"
	stationmem "github.com/enterprise-labs/seismic-event-associator/internal/station/infrastructure"
	travelapp "github.com/enterprise-labs/seismic-event-associator/internal/traveltime/application"
	waveformmem "github.com/enterprise-labs/seismic-event-associator/internal/waveform/infrastructure"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"
)

type Runtime struct {
	Config   platform.Config
	Server   *http.Server
	Stations *stationmem.Repository
	Picks    *pickermem.Repository
	Events   *assocmem.Repository
	Evidence *evidencemem.Repository
	ready    atomic.Bool
}

func NewRuntime(ctx context.Context, cfg platform.Config, logger *slog.Logger) (*Runtime, error) {
	clock := clockpkg.RealClock{}
	stations := stationmem.NewRepository()
	for _, s := range DefaultStations() {
		if err := stations.Upsert(ctx, s); err != nil {
			return nil, fmt.Errorf("bootstrap station: %w", err)
		}
	}
	picks := pickermem.NewRepository()
	events := assocmem.NewRepository()
	evidenceRepo := evidencemem.NewRepository()
	evidence := evidenceapp.NewRecorder(evidenceRepo, clock)
	model, err := travelapp.NewHomogeneous(6.0, 3.5, "homogeneous-v1")
	if err != nil {
		return nil, err
	}
	locator := assocapp.NewLocator(picks, stations, model, events, clock, "grid-associator-v1")
	lifecycle := assocapp.NewLifecycle(events, clock)
	mag := magnitudeapp.NewService(magnitude.NewLocalMagnitude("ml-v1"))
	stationList, _ := stations.List(ctx)
	mag.SetStations(stationList)
	detector := pickerapp.NewDetector(picks, clock, "sta-lta-picker-v1")
	jobs := jobapp.NewService(jobmem.NewRepository(), clock, cfg.NodeID)
	metrics := platform.NewMetrics()
	parser := seedapp.NewParser(seedapp.SampleDecoder{}, 1<<20)
	runtime := &Runtime{Config: cfg, Stations: stations, Picks: picks, Events: events, Evidence: evidenceRepo}
	api := adapter.NewAPI(adapter.Dependencies{Stations: stations, Parser: parser, Pipeline: signalapp.NewPipeline(), Detector: detector, Picks: picks, Locator: locator, Events: events, Lifecycle: lifecycle, Magnitude: mag, Evidence: evidence, Jobs: jobs, Metrics: metrics, Clock: clock, Ready: runtime.ready.Load, Objects: waveformmem.NewObjectStore(64 << 20)})
	middleware := adapter.NewMiddleware(cfg, logger, metrics)
	runtime.Server = &http.Server{Addr: cfg.Address, Handler: middleware.Wrap(api.Routes()), ReadHeaderTimeout: 4 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32 << 10}
	runtime.ready.Store(true)
	return runtime, nil
}
func (r *Runtime) Shutdown(ctx context.Context) error {
	r.ready.Store(false)
	return r.Server.Shutdown(ctx)
}
func DefaultStations() []station.Station {
	base := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	return []station.Station{{Network: "XX", Code: "STA1", Latitude: 0, Longitude: -0.5, Elevation: 50, Enabled: true, Version: 1, ValidFrom: base}, {Network: "XX", Code: "STA2", Latitude: 0.5, Longitude: 0, Elevation: 80, Enabled: true, Version: 1, ValidFrom: base}, {Network: "XX", Code: "STA3", Latitude: 0, Longitude: 0.5, Elevation: 20, Enabled: true, Version: 1, ValidFrom: base}, {Network: "XX", Code: "STA4", Latitude: -0.5, Longitude: 0, Elevation: 10, Enabled: true, Version: 1, ValidFrom: base}}
}
