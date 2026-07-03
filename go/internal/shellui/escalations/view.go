package escalations

import (
	"context"
	"time"

	"agora-de.local/go/internal/osboundary"
)

type PendingView struct {
	ID           string
	RequesterUID uint32
	Summary      string
	Age          time.Duration
}

type Clock interface {
	Now() time.Time
}

type SystemClock struct{}

func (SystemClock) Now() time.Time {
	return time.Now()
}

func ListPendingViews(
	ctx context.Context,
	client osboundary.EscalationClient,
	clock Clock,
) ([]PendingView, error) {
	events, err := client.ListPendingEscalations(ctx)
	if err != nil {
		return nil, err
	}
	now := clock.Now()
	views := make([]PendingView, 0, len(events))
	for _, event := range events {
		views = append(views, PendingView{
			ID:           event.ID,
			RequesterUID: event.RequesterUID,
			Summary:      event.Summary,
			Age:          now.Sub(event.CreatedAt),
		})
	}
	return views, nil
}

