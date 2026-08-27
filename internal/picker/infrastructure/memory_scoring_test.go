package infrastructure

import (
	"context"
	picker "github.com/enterprise-labs/seismic-event-associator/internal/picker/domain"
	"testing"
	"time"
)

func savePickForTest(t *testing.T, r *Repository, id string, at time.Time) {
	t.Helper()
	if err := r.Save(context.Background(), picker.PickWithEvidence{Pick: picker.Pick{ID: id, Time: at}}); err != nil {
		t.Fatal(err)
	}
}

func TestRepositoryListRespectsTimeWindow(t *testing.T) {
	r := NewRepository()
	savePickForTest(t, r, "early", time.Unix(1, 0))
	savePickForTest(t, r, "late", time.Unix(2, 0))
	items, err := r.List(context.Background(), time.Unix(2, 0), time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Pick.ID != "late" {
		t.Fatalf("expected only late pick, got %v", items)
	}
}

func TestRepositoryListReturnsIndependentCopy(t *testing.T) {
	r := NewRepository()
	savePickForTest(t, r, "a", time.Unix(1, 0))
	savePickForTest(t, r, "b", time.Unix(2, 0))
	first, err := r.List(context.Background(), time.Time{}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	first[0].Pick.ID = "corrupted"
	second, err := r.List(context.Background(), time.Time{}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if second[0].Pick.ID == "corrupted" {
		t.Fatal("List returned aliased internal snapshot")
	}
}

func TestRepositorySaveReplacesExistingSnapshotEntry(t *testing.T) {
	r := NewRepository()
	savePickForTest(t, r, "same", time.Unix(1, 0))
	savePickForTest(t, r, "same", time.Unix(2, 0))
	items, err := r.List(context.Background(), time.Time{}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one entry after re-save, got %d: %v", len(items), items)
	}
	if items[0].Pick.Time.Unix() != 2 {
		t.Fatalf("expected latest snapshot value, got %v", items[0].Pick.Time)
	}
}

func TestRepositoryUpdateStatusReflectedInSnapshot(t *testing.T) {
	r := NewRepository()
	savePickForTest(t, r, "a", time.Unix(1, 0))
	if err := r.UpdateStatus(context.Background(), "a", "reviewed"); err != nil {
		t.Fatal(err)
	}
	items, err := r.List(context.Background(), time.Time{}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Pick.Status != "reviewed" {
		t.Fatalf("expected updated status in list, got %v", items)
	}
}
