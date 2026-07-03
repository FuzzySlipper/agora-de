package audittail

import (
	"context"
	"errors"
	"testing"
	"time"

	"agora-de.local/go/internal/osboundary"
)

func TestCollectViewsUsesTypedAuditClient(t *testing.T) {
	createdAt := time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC)
	client := fakeAuditClient{
		events: []osboundary.AuditEvent{
			{
				ID:        "audit-1",
				ActorUID:  60001,
				Action:    "grant.recorded",
				Subject:   "view-42",
				CreatedAt: createdAt,
			},
		},
	}

	views, err := CollectViews(context.Background(), client, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(views) != 1 {
		t.Fatalf("views = %d, want 1", len(views))
	}
	if views[0].ID != "audit-1" {
		t.Fatalf("view id = %q, want audit-1", views[0].ID)
	}
	if views[0].Subject != "view-42" {
		t.Fatalf("subject = %q, want view-42", views[0].Subject)
	}
}

func TestCollectViewsPropagatesBoundaryError(t *testing.T) {
	_, err := CollectViews(context.Background(), fakeAuditClient{err: osboundary.ErrUnavailable}, 10)
	if !errors.Is(err, osboundary.ErrUnavailable) {
		t.Fatalf("error = %v, want ErrUnavailable", err)
	}
}

type fakeAuditClient struct {
	events []osboundary.AuditEvent
	err    error
}

func (client fakeAuditClient) StreamAuditEvents(context.Context) (<-chan osboundary.AuditEvent, <-chan error) {
	events := make(chan osboundary.AuditEvent, len(client.events))
	errs := make(chan error, 1)
	for _, event := range client.events {
		events <- event
	}
	close(events)
	if client.err != nil {
		errs <- client.err
	}
	close(errs)
	return events, errs
}

