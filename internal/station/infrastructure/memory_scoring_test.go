package infrastructure

import (
	"context"
	"errors"
	station "github.com/enterprise-labs/seismic-event-associator/internal/station/domain"
	"strings"
	"testing"
	"time"
)

func TestRepositoryGetPreservesNotFoundForMissingStation(t *testing.T) {
	r := NewRepository()
	_, err := r.Get(context.Background(), "XX.MISSING", time.Unix(100, 0))
	if !errors.Is(err, station.ErrStationNotFound) {
		t.Fatalf("err=%v", err)
	}
	if !strings.Contains(err.Error(), "missing") {
		t.Fatalf("err=%v", err)
	}
}

func TestRepositoryGetPreservesNotFoundForDisabledStation(t *testing.T) {
	r := NewRepository()
	item := station.Station{Network: "XX", Code: "OFF", Latitude: 1, Longitude: 1, Enabled: false, Version: 1, ValidFrom: time.Unix(0, 0)}
	if err := r.Upsert(context.Background(), item); err != nil {
		t.Fatal(err)
	}
	_, err := r.Get(context.Background(), item.ID(), time.Unix(100, 0))
	if !errors.Is(err, station.ErrStationNotFound) {
		t.Fatalf("err=%v", err)
	}
	if !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("err=%v", err)
	}
}

func TestRepositoryGetPreservesNotFoundForExpiredStation(t *testing.T) {
	r := NewRepository()
	end := time.Unix(50, 0)
	item := station.Station{Network: "XX", Code: "OLD", Latitude: 1, Longitude: 1, Enabled: true, Version: 1, ValidFrom: time.Unix(0, 0), ValidTo: &end}
	if err := r.Upsert(context.Background(), item); err != nil {
		t.Fatal(err)
	}
	_, err := r.Get(context.Background(), item.ID(), time.Unix(100, 0))
	if !errors.Is(err, station.ErrStationNotFound) {
		t.Fatalf("err=%v", err)
	}
	if !strings.Contains(err.Error(), "expired") {
		t.Fatalf("err=%v", err)
	}
}

func TestRepositoryGetPreservesNotFoundForNotYetValidStation(t *testing.T) {
	r := NewRepository()
	item := station.Station{Network: "XX", Code: "FUTURE", Latitude: 1, Longitude: 1, Enabled: true, Version: 1, ValidFrom: time.Unix(200, 0)}
	if err := r.Upsert(context.Background(), item); err != nil {
		t.Fatal(err)
	}
	_, err := r.Get(context.Background(), item.ID(), time.Unix(100, 0))
	if !errors.Is(err, station.ErrStationNotFound) || !strings.Contains(err.Error(), "not_yet_valid") {
		t.Fatalf("err=%v", err)
	}
}
