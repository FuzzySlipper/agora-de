package wayfireproto

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
)

type MessageType string

const (
	MessageTypeSurfaceEvent       MessageType = "surface_event"
	MessageTypeLayoutState        MessageType = "layout_state"
	MessageTypePolicyReplace      MessageType = "policy_replace"
	MessageTypePolicyUpsert       MessageType = "policy_upsert"
	MessageTypePolicyRemove       MessageType = "policy_remove"
	MessageTypeInputContext       MessageType = "input_context"
	MessageTypeCloseSurface       MessageType = "close_surface"
	MessageTypeCloseSurfacesByUID MessageType = "close_surfaces_by_uid"
	MessageTypePlaceSurface       MessageType = "place_surface"
	MessageTypePlaceResponse      MessageType = "place_response"
)

type SurfaceEventKind string

const (
	SurfaceEventMapped      SurfaceEventKind = "mapped"
	SurfaceEventUnmapped    SurfaceEventKind = "unmapped"
	SurfaceEventFocused     SurfaceEventKind = "focused"
	SurfaceEventInputDenied SurfaceEventKind = "input_denied"
)

type DeviceKind string

const (
	DeviceKeyboard DeviceKind = "keyboard"
	DevicePointer  DeviceKind = "pointer"
)

type SurfaceRef struct {
	ID            string `json:"id"`
	WayfireViewID int64  `json:"wayfire_view_id,omitempty"`
	AppID         string `json:"app_id,omitempty"`
	Title         string `json:"title,omitempty"`
	Role          string `json:"role,omitempty"`
}

type ClientCred struct {
	PID int `json:"pid"`
	UID int `json:"uid"`
	GID int `json:"gid"`
}

type SurfaceEventMessage struct {
	Type    MessageType      `json:"type"`
	Event   SurfaceEventKind `json:"event"`
	Device  DeviceKind       `json:"device,omitempty"`
	Surface SurfaceRef       `json:"surface"`
	Client  ClientCred       `json:"client"`
}

type SurfacePolicy struct {
	SurfaceID         string `json:"surface_id"`
	OwnerUID          int    `json:"owner_uid"`
	AllowPointerUIDs  []int  `json:"allow_pointer_uids,omitempty"`
	AllowKeyboardUIDs []int  `json:"allow_keyboard_uids,omitempty"`
}

type PolicyReplaceMessage struct {
	Type     MessageType     `json:"type"`
	Surfaces []SurfacePolicy `json:"surfaces"`
}

type PolicyUpsertMessage struct {
	Type    MessageType   `json:"type"`
	Surface SurfacePolicy `json:"surface"`
}

type PolicyRemoveMessage struct {
	Type      MessageType `json:"type"`
	SurfaceID string      `json:"surface_id"`
}

type InputContextMessage struct {
	Type     MessageType `json:"type"`
	ActorUID *int        `json:"actor_uid,omitempty"`
}

type CloseSurfaceMessage struct {
	Type      MessageType `json:"type"`
	SurfaceID string      `json:"surface_id"`
}

type CloseSurfacesByUIDMessage struct {
	Type     MessageType `json:"type"`
	OwnerUID int         `json:"owner_uid"`
}

func DecodeLine(line []byte) (MessageType, error) {
	var envelope struct {
		Type MessageType `json:"type"`
	}
	if err := json.Unmarshal(line, &envelope); err != nil {
		return "", fmt.Errorf("decode wayfire envelope: %w", err)
	}
	if !knownType(envelope.Type) {
		return "", fmt.Errorf("unknown wayfire message type %q", envelope.Type)
	}
	return envelope.Type, nil
}

func DecodeStream(reader io.Reader) ([]MessageType, error) {
	scanner := bufio.NewScanner(reader)
	var types []MessageType
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		messageType, err := DecodeLine(line)
		if err != nil {
			return nil, err
		}
		types = append(types, messageType)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan wayfire stream: %w", err)
	}
	return types, nil
}

func knownType(messageType MessageType) bool {
	switch messageType {
	case MessageTypeSurfaceEvent,
		MessageTypeLayoutState,
		MessageTypePolicyReplace,
		MessageTypePolicyUpsert,
		MessageTypePolicyRemove,
		MessageTypeInputContext,
		MessageTypeCloseSurface,
		MessageTypeCloseSurfacesByUID,
		MessageTypePlaceSurface,
		MessageTypePlaceResponse:
		return true
	default:
		return false
	}
}
