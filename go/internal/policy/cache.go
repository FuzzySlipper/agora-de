package policy

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"agora-de.local/go/internal/input"
	"agora-de.local/go/internal/wayfireproto"
)

var ErrUnsupportedCommand = errors.New("unsupported policy command")

type SurfacePolicy struct {
	SurfaceID         string
	OwnerUID          int
	AllowPointerUIDs  []int
	AllowKeyboardUIDs []int
}

type Cache struct {
	surfaces     map[string]SurfacePolicy
	inputContext input.Context
}

func NewCache() *Cache {
	return &Cache{
		surfaces:     map[string]SurfacePolicy{},
		inputContext: input.NewContext(),
	}
}

func (cache *Cache) ApplyBridgeCommandLine(line []byte) error {
	messageType, err := wayfireproto.DecodeLine(line)
	if err != nil {
		return err
	}

	switch messageType {
	case wayfireproto.MessageTypePolicyReplace:
		var command wayfireproto.PolicyReplaceMessage
		if err := json.Unmarshal(line, &command); err != nil {
			return fmt.Errorf("decode policy_replace: %w", err)
		}
		next := map[string]SurfacePolicy{}
		for _, surface := range command.Surfaces {
			next[surface.SurfaceID] = fromWirePolicy(surface)
		}
		cache.surfaces = next
		return nil
	case wayfireproto.MessageTypePolicyUpsert:
		var command wayfireproto.PolicyUpsertMessage
		if err := json.Unmarshal(line, &command); err != nil {
			return fmt.Errorf("decode policy_upsert: %w", err)
		}
		cache.surfaces[command.Surface.SurfaceID] = fromWirePolicy(command.Surface)
		return nil
	case wayfireproto.MessageTypePolicyRemove:
		var command wayfireproto.PolicyRemoveMessage
		if err := json.Unmarshal(line, &command); err != nil {
			return fmt.Errorf("decode policy_remove: %w", err)
		}
		delete(cache.surfaces, command.SurfaceID)
		return nil
	case wayfireproto.MessageTypeInputContext:
		var command wayfireproto.InputContextMessage
		if err := json.Unmarshal(line, &command); err != nil {
			return fmt.Errorf("decode input_context: %w", err)
		}
		if command.ActorUID == nil {
			cache.inputContext.ClearActorUID()
		} else {
			cache.inputContext.SetActorUID(*command.ActorUID)
		}
		return nil
	default:
		return fmt.Errorf("%w: %s", ErrUnsupportedCommand, messageType)
	}
}

func (cache *Cache) ApplyBridgeCommandStream(reader io.Reader) error {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		if err := cache.ApplyBridgeCommandLine(line); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan policy command stream: %w", err)
	}
	return nil
}

func (cache *Cache) SurfacePolicy(surfaceID string) (SurfacePolicy, bool) {
	policy, ok := cache.surfaces[surfaceID]
	return policy, ok
}

func (cache *Cache) ActorUID() (int, bool) {
	return cache.inputContext.ActorUID()
}

func fromWirePolicy(surface wayfireproto.SurfacePolicy) SurfacePolicy {
	return SurfacePolicy{
		SurfaceID:         surface.SurfaceID,
		OwnerUID:          surface.OwnerUID,
		AllowPointerUIDs:  append([]int(nil), surface.AllowPointerUIDs...),
		AllowKeyboardUIDs: append([]int(nil), surface.AllowKeyboardUIDs...),
	}
}
