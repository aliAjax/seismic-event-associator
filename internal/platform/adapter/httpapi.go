package adapter

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	assocapp "github.com/enterprise-labs/seismic-event-associator/internal/association/application"
	assoc "github.com/enterprise-labs/seismic-event-associator/internal/association/domain"
	evidenceapp "github.com/enterprise-labs/seismic-event-associator/internal/evidence/application"
	jobapp "github.com/enterprise-labs/seismic-event-associator/internal/job/application"
	magnitudeapp "github.com/enterprise-labs/seismic-event-associator/internal/magnitude/application"
	pickerapp "github.com/enterprise-labs/seismic-event-associator/internal/picker/application"
	picker "github.com/enterprise-labs/seismic-event-associator/internal/picker/domain"
	platform "github.com/enterprise-labs/seismic-event-associator/internal/platform/application"
	seedapp "github.com/enterprise-labs/seismic-event-associator/internal/seed/application"
	signalapp "github.com/enterprise-labs/seismic-event-associator/internal/signal/application"
	signal "github.com/enterprise-labs/seismic-event-associator/internal/signal/domain"
	station "github.com/enterprise-labs/seismic-event-associator/internal/station/domain"
	waveform "github.com/enterprise-labs/seismic-event-associator/internal/waveform/domain"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type API struct {
	stations  StationAPI
	parser    *seedapp.Parser
	pipeline  *signalapp.Pipeline
	detector  *pickerapp.Detector
	picks     picker.Repository
	locator   *assocapp.Locator
	events    assoc.Repository
	lifecycle *assocapp.Lifecycle
	magnitude *magnitudeapp.Service
	evidence  *evidenceapp.Recorder
	jobs      *jobapp.Service
	metrics   *platform.Metrics
	clock     interface{ Now() time.Time }
	ready     func() bool
	objects   waveform.ObjectStore
	seen      sync.Map
}
type StationAPI interface {
	Upsert(context.Context, station.Station) error
	Get(context.Context, string, time.Time) (station.Station, error)
	List(context.Context) ([]station.Station, error)
}
type Dependencies struct {
	Stations  StationAPI
	Parser    *seedapp.Parser
	Pipeline  *signalapp.Pipeline
	Detector  *pickerapp.Detector
	Picks     picker.Repository
	Locator   *assocapp.Locator
	Events    assoc.Repository
	Lifecycle *assocapp.Lifecycle
	Magnitude *magnitudeapp.Service
	Evidence  *evidenceapp.Recorder
	Jobs      *jobapp.Service
	Metrics   *platform.Metrics
	Clock     interface{ Now() time.Time }
	Ready     func() bool
	Objects   waveform.ObjectStore
}

func NewAPI(d Dependencies) *API {
	return &API{stations: d.Stations, parser: d.Parser, pipeline: d.Pipeline, detector: d.Detector, picks: d.Picks, locator: d.Locator, events: d.Events, lifecycle: d.Lifecycle, magnitude: d.Magnitude, evidence: d.Evidence, jobs: d.Jobs, metrics: d.Metrics, clock: d.Clock, ready: d.Ready, objects: d.Objects}
}
func (a *API) Routes() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("GET /healthz", a.health)
	m.HandleFunc("GET /readyz", a.readyz)
	m.HandleFunc("GET /metrics", a.metricsHandler)
	m.HandleFunc("POST /v1/stations", a.upsertStation)
	m.HandleFunc("GET /v1/stations", a.listStations)
	m.HandleFunc("POST /v1/waveforms", a.ingest)
	m.HandleFunc("POST /v1/associations", a.associate)
	m.HandleFunc("GET /v1/picks", a.listPicks)
	m.HandleFunc("GET /v1/events", a.listEvents)
	m.HandleFunc("GET /v1/events/{id}", a.getEvent)
	m.HandleFunc("GET /v1/events/{id}/versions", a.eventHistory)
	m.HandleFunc("POST /v1/events/{id}/revoke", a.revoke)
	m.HandleFunc("POST /v1/events/{id}/split", a.split)
	m.HandleFunc("GET /v1/evidence/{subject}", a.listEvidence)
	m.HandleFunc("POST /v1/jobs", a.submitJob)
	m.HandleFunc("GET /v1/jobs", a.listJobs)
	return m
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func errorJSON(w http.ResponseWriter, status int, e error) {
	writeJSON(w, status, map[string]any{"error": http.StatusText(status), "message": e.Error()})
}
func decode(r *http.Request, v any) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	if len(body) == 0 {
		return errors.New("empty body")
	}
	d := json.NewDecoder(strings.NewReader(string(body)))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return fmt.Errorf("decode JSON: %w", err)
	}
	return nil
}
func (a *API) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]string{"status": "healthy"})
}
func (a *API) readyz(w http.ResponseWriter, r *http.Request) {
	if a.ready != nil && !a.ready() {
		errorJSON(w, 503, errors.New("not ready"))
		return
	}
	writeJSON(w, 200, map[string]string{"status": "ready"})
}
func (a *API) metricsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	a.metrics.Write(w)
}
func (a *API) upsertStation(w http.ResponseWriter, r *http.Request) {
	var item station.Station
	if err := decode(r, &item); err != nil {
		errorJSON(w, 400, err)
		return
	}
	if err := a.stations.Upsert(r.Context(), item); err != nil {
		errorJSON(w, 400, err)
		return
	}
	writeJSON(w, 201, item)
}
func (a *API) listStations(w http.ResponseWriter, r *http.Request) {
	items, err := a.stations.List(r.Context())
	if err != nil {
		errorJSON(w, 500, err)
		return
	}
	writeJSON(w, 200, map[string]any{"stations": items})
}

type ingestRequest struct {
	Source string `json:"source"`
	Blocks []struct {
		Channel  waveform.Channel `json:"channel"`
		Start    time.Time        `json:"start"`
		Rate     float64          `json:"rate"`
		Samples  []float64        `json:"samples"`
		Sequence uint64           `json:"sequence"`
	} `json:"blocks"`
	MiniSEEDBase64 string            `json:"miniseed_base64,omitempty"`
	Parameters     signal.Parameters `json:"parameters"`
}

func (a *API) ingest(w http.ResponseWriter, r *http.Request) {
	var req ingestRequest
	if err := decode(r, &req); err != nil {
		errorJSON(w, 400, err)
		return
	}
	if req.Parameters.Version == "" {
		req.Parameters = signal.Parameters{Version: "signal-v1", Detrend: true, MeanRemove: true, FilterLowHz: 0.5, FilterHighHz: 12, FilterOrder: 2, STA: 500 * time.Millisecond, LTA: 5 * time.Second, TriggerOn: 2, TriggerOff: 1.2, MinTrigger: 100 * time.Millisecond, MaxTrigger: 10 * time.Second}
	}
	blocks := []waveform.SampleBlock{}
	for _, b := range req.Blocks {
		blocks = append(blocks, waveform.SampleBlock{Channel: b.Channel, Start: b.Start, Rate: b.Rate, Samples: b.Samples, Sequence: b.Sequence})
	}
	if req.MiniSEEDBase64 != "" {
		raw, err := base64.StdEncoding.DecodeString(req.MiniSEEDBase64)
		if err != nil {
			errorJSON(w, 400, err)
			return
		}
		if a.parser == nil {
			errorJSON(w, 500, errors.New("miniSEED parser unavailable"))
			return
		}
		parsed, err := a.parser.ParseStream(r.Context(), bytes.NewReader(raw), req.Source)
		if err != nil {
			errorJSON(w, 400, err)
			return
		}
		blocks = append(blocks, parsed.Blocks...)
		if a.objects != nil {
			sum := sha256.Sum256(raw)
			key := "miniseed/" + hex.EncodeToString(sum[:])
			if err := a.objects.Put(r.Context(), key, bytes.NewReader(raw), int64(len(raw))); err != nil {
				errorJSON(w, 500, err)
				return
			}
		}
	}
	if len(blocks) == 0 {
		errorJSON(w, 400, errors.New("blocks required"))
		return
	}
	result := []picker.PickWithEvidence{}
	for _, block := range blocks {
		digest := block.Digest()
		if _, exists := a.seen.Load(digest); exists {
			errorJSON(w, http.StatusConflict, waveform.ErrDuplicate)
			return
		}
		trace, err := a.pipeline.Process(block, req.Parameters)
		if err != nil {
			errorJSON(w, 400, err)
			return
		}
		picks, err := a.detector.Detect(r.Context(), trace, digest)
		if err != nil {
			errorJSON(w, 422, err)
			return
		}
		result = append(result, picks...)
		a.seen.Store(digest, struct{}{})
		a.metrics.Waveform()
		for _, p := range picks {
			a.metrics.Pick()
			_, _ = a.evidence.Record(r.Context(), "pick_evidence", p.Pick.ID, p.Pick.AlgorithmVersion, map[string]any{"parameters": p.Pick.ParameterVersion}, []string{p.Pick.RecordDigest}, p.Evidence)
		}
	}
	writeJSON(w, 201, map[string]any{"picks": result, "count": len(result)})
}

type associationRequest struct {
	From          time.Time  `json:"from"`
	To            time.Time  `json:"to"`
	Grid          assoc.Grid `json:"grid"`
	MinStations   int        `json:"min_stations"`
	MaxResidualMS int64      `json:"max_residual_ms"`
}

func (a *API) associate(w http.ResponseWriter, r *http.Request) {
	var req associationRequest
	if err := decode(r, &req); err != nil {
		errorJSON(w, 400, err)
		return
	}
	if req.Grid.MaxNodes == 0 {
		req.Grid = assoc.Grid{MinLatitude: -1, MaxLatitude: 1, MinLongitude: -1, MaxLongitude: 1, HorizontalStep: 1, DepthsKM: []float64{5, 15}, MaxNodes: 100}
	}
	event, err := a.locator.Associate(r.Context(), assoc.Request{From: req.From, To: req.To, Grid: req.Grid, MinStations: req.MinStations, MaxResidual: time.Duration(req.MaxResidualMS) * time.Millisecond})
	if err != nil {
		errorJSON(w, 422, err)
		return
	}
	if a.magnitude != nil {
		if estimate, magErr := a.magnitude.Estimate(r.Context(), event); magErr == nil {
			previousVersion := event.Version
			magnitudeapp.MergeIntoEvent(&event, estimate, a.clock.Now())
			event.Version++
			event.Supersedes = append(event.Supersedes, fmt.Sprintf("%s@%d", event.ID, previousVersion))
			event.Reason = "magnitude estimate appended"
			_, _ = a.evidence.Record(r.Context(), "magnitude_evidence", event.ID, estimate.Version, nil, nil, estimate)
			if saveErr := a.events.Save(r.Context(), event); saveErr != nil {
				errorJSON(w, 500, saveErr)
				return
			}
		}
	}
	_, _ = a.evidence.Record(r.Context(), "association_evidence", event.ID, event.AlgorithmVersion, map[string]any{"grid": req.Grid}, nil, event)
	a.metrics.Event()
	writeJSON(w, 201, event)
}
func (a *API) listPicks(w http.ResponseWriter, r *http.Request) {
	from, _ := time.Parse(time.RFC3339Nano, r.URL.Query().Get("from"))
	to, _ := time.Parse(time.RFC3339Nano, r.URL.Query().Get("to"))
	items, err := a.picks.List(r.Context(), from, to)
	if err != nil {
		errorJSON(w, 500, err)
		return
	}
	writeJSON(w, 200, map[string]any{"picks": items, "count": len(items)})
}
func (a *API) listEvents(w http.ResponseWriter, r *http.Request) {
	items, err := a.events.List(r.Context())
	if err != nil {
		errorJSON(w, 500, err)
		return
	}
	writeJSON(w, 200, map[string]any{"events": items, "count": len(items)})
}
func (a *API) getEvent(w http.ResponseWriter, r *http.Request) {
	item, err := a.events.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		errorJSON(w, 404, err)
		return
	}
	writeJSON(w, 200, item)
}
func (a *API) eventHistory(w http.ResponseWriter, r *http.Request) {
	items, err := a.events.History(r.Context(), r.PathValue("id"))
	if err != nil {
		errorJSON(w, 404, err)
		return
	}
	writeJSON(w, 200, map[string]any{"versions": items})
}
func (a *API) revoke(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Reason string `json:"reason"`
	}
	if err := decode(r, &req); err != nil {
		errorJSON(w, 400, err)
		return
	}
	item, err := a.lifecycle.Revoke(r.Context(), r.PathValue("id"), req.Reason)
	if err != nil {
		errorJSON(w, 409, err)
		return
	}
	writeJSON(w, 200, item)
}
func (a *API) split(w http.ResponseWriter, r *http.Request) {
	items, err := a.lifecycle.Split(r.Context(), r.PathValue("id"))
	if err != nil {
		errorJSON(w, 409, err)
		return
	}
	writeJSON(w, 200, map[string]any{"events": items})
}
func (a *API) listEvidence(w http.ResponseWriter, r *http.Request) {
	items, err := a.evidence.ForSubject(r.Context(), r.PathValue("subject"))
	if err != nil {
		errorJSON(w, 500, err)
		return
	}
	writeJSON(w, 200, map[string]any{"evidence": items})
}
func (a *API) submitJob(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Type        string `json:"type"`
		InputDigest string `json:"input_digest"`
		Priority    int    `json:"priority"`
		MaxAttempts int    `json:"max_attempts"`
	}
	if err := decode(r, &req); err != nil {
		errorJSON(w, 400, err)
		return
	}
	item, err := a.jobs.Submit(r.Context(), req.Type, req.InputDigest, req.Priority, req.MaxAttempts)
	if err != nil {
		errorJSON(w, 400, err)
		return
	}
	writeJSON(w, 201, item)
}
func (a *API) listJobs(w http.ResponseWriter, r *http.Request) {
	items, err := a.jobs.List(r.Context())
	if err != nil {
		errorJSON(w, 500, err)
		return
	}
	writeJSON(w, 200, map[string]any{"jobs": items})
}
func parseInt(value string, defaultValue int) int {
	n, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}
	return n
}
