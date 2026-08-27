package application

import (
	"context"
	assoc "github.com/enterprise-labs/seismic-event-associator/internal/association/domain"
	assocmem "github.com/enterprise-labs/seismic-event-associator/internal/association/infrastructure"
	picker "github.com/enterprise-labs/seismic-event-associator/internal/picker/domain"
	pickermem "github.com/enterprise-labs/seismic-event-associator/internal/picker/infrastructure"
	clockpkg "github.com/enterprise-labs/seismic-event-associator/internal/platform/domain"
	station "github.com/enterprise-labs/seismic-event-associator/internal/station/domain"
	stationmem "github.com/enterprise-labs/seismic-event-associator/internal/station/infrastructure"
	travelapp "github.com/enterprise-labs/seismic-event-associator/internal/traveltime/application"
	travel "github.com/enterprise-labs/seismic-event-associator/internal/traveltime/domain"
	"testing"
	"time"
)

func TestLocatorRecoversKnownGridNode(t *testing.T) {
	ctx := context.Background()
	clock := &clockpkg.ManualClock{Current: time.Unix(2000, 0)}
	stations := stationmem.NewRepository()
	items := []station.Station{{Network: "XX", Code: "A", Latitude: 0, Longitude: -.5, Enabled: true, Version: 1, ValidFrom: time.Unix(0, 0)}, {Network: "XX", Code: "B", Latitude: .5, Longitude: 0, Enabled: true, Version: 1, ValidFrom: time.Unix(0, 0)}, {Network: "XX", Code: "C", Latitude: 0, Longitude: .5, Enabled: true, Version: 1, ValidFrom: time.Unix(0, 0)}, {Network: "XX", Code: "D", Latitude: -.5, Longitude: 0, Enabled: true, Version: 1, ValidFrom: time.Unix(0, 0)}}
	model, _ := travelapp.NewHomogeneous(6, 3.5, "test")
	origin := time.Unix(1000, 0)
	source := travel.Point{Latitude: 0, Longitude: 0, DepthKM: 5}
	repo := pickermem.NewRepository()
	for i, s := range items {
		if err := stations.Upsert(ctx, s); err != nil {
			t.Fatal(err)
		}
		tt, _ := model.TravelTime(ctx, "P", source, s)
		p := picker.PickWithEvidence{Pick: picker.Pick{ID: string(rune('a' + i)), StationID: s.ID(), Phase: "P", Time: origin.Add(time.Duration(tt * float64(time.Second))), UncertaintyMS: 20, Amplitude: 10, Status: "active"}}
		if err := repo.Save(ctx, p); err != nil {
			t.Fatal(err)
		}
	}
	locator := NewLocator(repo, stations, model, assocmem.NewRepository(), clock, "test")
	event, err := locator.Associate(ctx, assoc.Request{From: origin.Add(-time.Second), To: origin.Add(20 * time.Second), Grid: assoc.Grid{MinLatitude: -.1, MaxLatitude: .1, MinLongitude: -.1, MaxLongitude: .1, HorizontalStep: .1, DepthsKM: []float64{0, 5, 10}, MaxNodes: 100}, MinStations: 4, MaxResidual: 200 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	if event.Hypocenter.Latitude != 0 || event.Hypocenter.Longitude != 0 || len(event.Picks) != 4 {
		t.Fatalf("event=%+v", event)
	}
}
