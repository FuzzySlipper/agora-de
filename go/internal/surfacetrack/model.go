package surfacetrack

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"

	"agora-de.local/go/internal/wayfireproto"
)

type SurfaceState struct {
	ID            string
	WayfireViewID int64
	AppID         string
	Title         string
	Role          string
	OwnerUID      int
	PID           int
	GID           int
	Mapped        bool
	Focused       bool
	DeniedInputs  int
}

type Projection struct {
	surfaces  map[string]SurfaceState
	focusedID string
}

func NewProjection() *Projection {
	return &Projection{surfaces: map[string]SurfaceState{}}
}

func (projection *Projection) ApplyWayfireEvent(event wayfireproto.SurfaceEventMessage) error {
	if event.Type != wayfireproto.MessageTypeSurfaceEvent {
		return fmt.Errorf("surfacetrack accepts only surface_event messages, got %q", event.Type)
	}
	if event.Surface.ID == "" {
		return fmt.Errorf("surface event missing surface id")
	}

	state := projection.surfaces[event.Surface.ID]
	state.ID = event.Surface.ID
	state.WayfireViewID = event.Surface.WayfireViewID
	state.AppID = event.Surface.AppID
	state.Title = event.Surface.Title
	state.Role = event.Surface.Role
	state.OwnerUID = event.Client.UID
	state.PID = event.Client.PID
	state.GID = event.Client.GID

	switch event.Event {
	case wayfireproto.SurfaceEventMapped:
		state.Mapped = true
	case wayfireproto.SurfaceEventFocused:
		state.Mapped = true
		projection.clearFocus()
		state.Focused = true
		projection.focusedID = state.ID
	case wayfireproto.SurfaceEventUnmapped:
		state.Mapped = false
		state.Focused = false
		if projection.focusedID == state.ID {
			projection.focusedID = ""
		}
	case wayfireproto.SurfaceEventInputDenied:
		state.DeniedInputs++
	default:
		return fmt.Errorf("unknown surface event kind %q", event.Event)
	}

	projection.surfaces[state.ID] = state
	return nil
}

func (projection *Projection) Surface(id string) (SurfaceState, bool) {
	state, ok := projection.surfaces[id]
	return state, ok
}

func (projection *Projection) FocusedSurfaceID() string {
	return projection.focusedID
}

func DecodeWayfireSurfaceEvents(reader io.Reader) ([]wayfireproto.SurfaceEventMessage, error) {
	scanner := bufio.NewScanner(reader)
	var events []wayfireproto.SurfaceEventMessage
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var event wayfireproto.SurfaceEventMessage
		if err := json.Unmarshal(line, &event); err != nil {
			return nil, fmt.Errorf("decode surface event: %w", err)
		}
		if event.Type != wayfireproto.MessageTypeSurfaceEvent {
			return nil, fmt.Errorf("expected surface_event, got %q", event.Type)
		}
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan surface events: %w", err)
	}
	return events, nil
}

func (projection *Projection) clearFocus() {
	if projection.focusedID == "" {
		return
	}
	state, ok := projection.surfaces[projection.focusedID]
	if !ok {
		projection.focusedID = ""
		return
	}
	state.Focused = false
	projection.surfaces[state.ID] = state
	projection.focusedID = ""
}
