package surfaceactions

import (
	"encoding/json"
	"errors"
	"fmt"

	"agora-de.local/go/internal/wayfireproto"
)

var ErrUnsupportedAction = errors.New("unsupported surface action")

type ActionKind string

const (
	ActionCloseSurface       ActionKind = "close_surface"
	ActionCloseSurfacesByUID ActionKind = "close_surfaces_by_uid"
)

type Action struct {
	Kind      ActionKind
	SurfaceID string
	OwnerUID  int
}

func DecodeBridgeActionLine(line []byte) (Action, error) {
	messageType, err := wayfireproto.DecodeLine(line)
	if err != nil {
		return Action{}, err
	}

	switch messageType {
	case wayfireproto.MessageTypeCloseSurface:
		var command wayfireproto.CloseSurfaceMessage
		if err := json.Unmarshal(line, &command); err != nil {
			return Action{}, fmt.Errorf("decode close_surface: %w", err)
		}
		if command.SurfaceID == "" {
			return Action{}, fmt.Errorf("close_surface missing surface_id")
		}
		return Action{Kind: ActionCloseSurface, SurfaceID: command.SurfaceID}, nil
	case wayfireproto.MessageTypeCloseSurfacesByUID:
		var command wayfireproto.CloseSurfacesByUIDMessage
		if err := json.Unmarshal(line, &command); err != nil {
			return Action{}, fmt.Errorf("decode close_surfaces_by_uid: %w", err)
		}
		if command.OwnerUID == 0 {
			return Action{}, fmt.Errorf("close_surfaces_by_uid missing owner_uid")
		}
		return Action{Kind: ActionCloseSurfacesByUID, OwnerUID: command.OwnerUID}, nil
	default:
		return Action{}, fmt.Errorf("%w: %s", ErrUnsupportedAction, messageType)
	}
}

