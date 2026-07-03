package audittail

import (
	"context"
	"time"

	"agora-de.local/go/internal/osboundary"
)

type EventView struct {
	ID        string
	ActorUID  uint32
	Action    string
	Subject   string
	CreatedAt time.Time
}

func CollectViews(ctx context.Context, client osboundary.AuditClient, limit int) ([]EventView, error) {
	if limit <= 0 {
		return nil, nil
	}
	events, errs := client.StreamAuditEvents(ctx)
	views := make([]EventView, 0, limit)
	for len(views) < limit {
		select {
		case event, ok := <-events:
			if !ok {
				return views, nil
			}
			views = append(views, EventView{
				ID:        event.ID,
				ActorUID:  event.ActorUID,
				Action:    event.Action,
				Subject:   event.Subject,
				CreatedAt: event.CreatedAt,
			})
		case err, ok := <-errs:
			if !ok {
				errs = nil
				continue
			}
			if err != nil {
				return nil, err
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return views, nil
}

