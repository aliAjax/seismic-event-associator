package application

import (
	"context"
	"fmt"
	assoc "github.com/enterprise-labs/seismic-event-associator/internal/association/domain"
	magnitude "github.com/enterprise-labs/seismic-event-associator/internal/magnitude/domain"
	station "github.com/enterprise-labs/seismic-event-associator/internal/station/domain"
	"sync"
	"time"
)

type Service struct {
	estimator magnitude.Estimator
	stations  map[string]station.Station
	mu        sync.RWMutex
}

func NewService(estimator magnitude.Estimator) *Service {
	return &Service{estimator: estimator, stations: map[string]station.Station{}}
}
func (s *Service) SetStations(items []station.Station) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stations = map[string]station.Station{}
	for _, item := range items {
		s.stations[item.ID()] = item
	}
}
func (s *Service) Estimate(ctx context.Context, event assoc.Event) (magnitude.Estimate, error) {
	s.mu.RLock()
	stations := map[string]station.Station{}
	for k, v := range s.stations {
		stations[k] = v
	}
	s.mu.RUnlock()
	estimate, err := s.estimator.Estimate(ctx, event, stations)
	if err != nil {
		return estimate, fmt.Errorf("estimate magnitude: %w", err)
	}
	return estimate, nil
}
func MergeIntoEvent(event *assoc.Event, estimate magnitude.Estimate, now time.Time) {
	event.Magnitude = &estimate.Value
	event.UpdatedAt = now
}
