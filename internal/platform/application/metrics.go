package application

import (
	"fmt"
	"io"
	"sync/atomic"
	"time"
)

type Metrics struct {
	requests  atomic.Uint64
	errors    atomic.Uint64
	waveforms atomic.Uint64
	picks     atomic.Uint64
	events    atomic.Uint64
	started   time.Time
}

func NewMetrics() *Metrics { return &Metrics{started: time.Now()} }
func (m *Metrics) Request(err bool) {
	m.requests.Add(1)
	if err {
		m.errors.Add(1)
	}
}
func (m *Metrics) Waveform() { m.waveforms.Add(1) }
func (m *Metrics) Pick()     { m.picks.Add(1) }
func (m *Metrics) Event()    { m.events.Add(1) }
func (m *Metrics) Write(w io.Writer) {
	fmt.Fprintf(w, "# TYPE seismic_http_requests_total counter\nseismic_http_requests_total %d\nseismic_http_errors_total %d\nseismic_waveforms_total %d\nseismic_picks_total %d\nseismic_events_total %d\nseismic_uptime_seconds %.0f\n", m.requests.Load(), m.errors.Load(), m.waveforms.Load(), m.picks.Load(), m.events.Load(), time.Since(m.started).Seconds())
}
