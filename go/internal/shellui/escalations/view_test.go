package escalations

import (
	"context"
	"errors"
	"testing"
	"time"

	"agora-de.local/go/internal/osboundary"
)

func TestListPendingViewsUsesTypedBoundaryClient(t *testing.T) {
	now := time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC)
	client := fakeEscalationClient{
		pending: []osboundary.AdminEscalationEvent{
			{
				ID:           "esc-1",
				RequesterUID: 60001,
				Summary:      "Need approval",
				CreatedAt:    now.Add(-5 * time.Minute),
			},
		},
	}

	views, err := ListPendingViews(context.Background(), client, fixedClock{now: now})
	if err != nil {
		t.Fatal(err)
	}
	if len(views) != 1 {
		t.Fatalf("views = %d, want 1", len(views))
	}
	if views[0].ID != "esc-1" {
		t.Fatalf("view id = %q, want esc-1", views[0].ID)
	}
	if views[0].Age != 5*time.Minute {
		t.Fatalf("age = %s, want 5m", views[0].Age)
	}
}

func TestListPendingViewsPropagatesBoundaryError(t *testing.T) {
	_, err := ListPendingViews(
		context.Background(),
		fakeEscalationClient{err: osboundary.ErrUnavailable},
		fixedClock{now: time.Now()},
	)
	if !errors.Is(err, osboundary.ErrUnavailable) {
		t.Fatalf("error = %v, want ErrUnavailable", err)
	}
}

type fixedClock struct {
	now time.Time
}

func (clock fixedClock) Now() time.Time {
	return clock.now
}

type fakeEscalationClient struct {
	pending []osboundary.AdminEscalationEvent
	err     error
}

func (client fakeEscalationClient) ListPendingEscalations(context.Context) ([]osboundary.AdminEscalationEvent, error) {
	return client.pending, client.err
}

func (client fakeEscalationClient) SubmitDecision(context.Context, osboundary.EscalationDecisionRequest) (osboundary.EscalationDecisionResponse, error) {
	return osboundary.EscalationDecisionResponse{}, nil
}

