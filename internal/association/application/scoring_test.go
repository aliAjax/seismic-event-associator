package application

import (
	"context"
	"testing"
	"time"
	assoc "github.com/enterprise-labs/seismic-event-associator/internal/association/domain"
	assocmem "github.com/enterprise-labs/seismic-event-associator/internal/association/infrastructure"
)

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

func makeEvent(id string, status assoc.Status, picks int) assoc.Event {
	p := make([]assoc.AssociatedPick, picks)
	for i := range p {
		p[i].PickID = id
	}
	return assoc.Event{ID: id, Version: 1, Status: status, Picks: p, CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0)}
}

func TestRevokedEventCannotBeSplit(t *testing.T) {
	repo := assocmem.NewRepository()
	lc := NewLifecycle(repo, fixedClock{now: time.Unix(10, 0)})
	if err := repo.Save(context.Background(), makeEvent("e1", assoc.Revoked, 6)); err != nil {
		t.Fatal(err)
	}
	if _, err := lc.Split(context.Background(), "e1"); err == nil {
		t.Fatal("expected split of revoked event to fail")
	}
}

func TestRevokedEventCannotBeMerged(t *testing.T) {
	repo := assocmem.NewRepository()
	lc := NewLifecycle(repo, fixedClock{now: time.Unix(10, 0)})
	if err := repo.Save(context.Background(), makeEvent("e1", assoc.Confirmed, 3)); err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(context.Background(), makeEvent("e2", assoc.Revoked, 3)); err != nil {
		t.Fatal(err)
	}
	if _, err := lc.Merge(context.Background(), []string{"e1", "e2"}); err == nil {
		t.Fatal("expected merge with revoked event to fail")
	}
}
