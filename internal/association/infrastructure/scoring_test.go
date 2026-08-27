package infrastructure

import (
	"context"
	"testing"
	assoc "github.com/enterprise-labs/seismic-event-associator/internal/association/domain"
)

func TestRepositorySaveRejectsRevokedToActiveTransition(t *testing.T) {
	r := NewRepository()
	old := assoc.Event{ID: "e1", Version: 1, Status: assoc.Revoked}
	if err := r.Save(context.Background(), old); err != nil {
		t.Fatal(err)
	}
	next := old
	next.Version = 2
	next.Status = assoc.Confirmed
	if err := r.Save(context.Background(), next); err == nil {
		t.Fatal("expected reactivation of revoked event to fail")
	}
}
