package application

import (
	"context"
	"fmt"
	assoc "github.com/enterprise-labs/seismic-event-associator/internal/association/domain"
	"sort"
	"time"
)

type Lifecycle struct {
	events assoc.Repository
	clock  Clock
}

func NewLifecycle(events assoc.Repository, clock Clock) *Lifecycle {
	return &Lifecycle{events: events, clock: clock}
}
func (l *Lifecycle) Revoke(ctx context.Context, id, reason string) (assoc.Event, error) {
	event, err := l.events.Get(ctx, id)
	if err != nil {
		return event, err
	}
	if event.Status == assoc.Revoked {
		return event, nil
	}
	event.Version++
	event.Status = assoc.Revoked
	event.Reason = reason
	event.UpdatedAt = l.clock.Now()
	event.Supersedes = append(event.Supersedes, fmt.Sprintf("%s@%d", id, event.Version-1))
	return event, l.events.Save(ctx, event)
}
func (l *Lifecycle) Split(ctx context.Context, id string) ([]assoc.Event, error) {
	event, err := l.events.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if len(event.Picks) < 6 {
		return nil, fmt.Errorf("event needs at least six picks to split")
	}
	picks := append([]assoc.AssociatedPick(nil), event.Picks...)
	sort.Slice(picks, func(i, j int) bool { return picks[i].ResidualMS < picks[j].ResidualMS })
	mid := len(picks) / 2
	if mid < 3 || len(picks)-mid < 3 {
		return nil, fmt.Errorf("split would produce undersized event")
	}
	now := l.clock.Now()
	children := []assoc.Event{}
	for index, group := range [][]assoc.AssociatedPick{picks[:mid], picks[mid:]} {
		child := event
		child.ID = fmt.Sprintf("%s-s%d", event.ID, index+1)
		child.Version = 1
		child.Picks = append([]assoc.AssociatedPick(nil), group...)
		child.Status = assoc.Split
		child.CreatedAt = now
		child.UpdatedAt = now
		child.Supersedes = []string{fmt.Sprintf("%s@%d", event.ID, event.Version)}
		child.Reason = "residual cluster split"
		if err := l.events.Save(ctx, child); err != nil {
			return children, err
		}
		children = append(children, child)
	}
	event.Version++
	event.Status = assoc.Merged
	event.Reason = "superseded by split children"
	event.UpdatedAt = now
	if err := l.events.Save(ctx, event); err != nil {
		return children, err
	}
	return children, nil
}
func (l *Lifecycle) Merge(ctx context.Context, ids []string) (assoc.Event, error) {
	if len(ids) < 2 {
		return assoc.Event{}, fmt.Errorf("at least two events required")
	}
	events := make([]assoc.Event, 0, len(ids))
	for _, id := range ids {
		e, err := l.events.Get(ctx, id)
		if err != nil {
			return assoc.Event{}, err
		}
		events = append(events, e)
	}
	base := events[0]
	base.Version++
	base.Status = assoc.Confirmed
	base.UpdatedAt = l.clock.Now()
	base.Reason = "operator merge"
	var picks []assoc.AssociatedPick
	for _, e := range events {
		picks = mergePicks(picks, e.Picks)
		base.Supersedes = append(base.Supersedes, fmt.Sprintf("%s@%d", e.ID, e.Version))
	}
	base.Picks = picks
	if err := l.events.Save(ctx, base); err != nil {
		return base, err
	}
	for _, e := range events[1:] {
		e.Version++
		e.Status = assoc.Merged
		e.UpdatedAt = l.clock.Now()
		e.Reason = "merged into " + base.ID
		if err := l.events.Save(ctx, e); err != nil {
			return base, err
		}
	}
	return base, nil
}
func absTime(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}
