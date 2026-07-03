package policy

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCacheAppliesPolicyCommandsFromFixture(t *testing.T) {
	file, err := os.Open(fixturePath(t, "bridge-commands.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	cache := NewCache()
	scanner := bufio.NewScanner(file)
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		if lineNumber > 5 {
			break
		}
		if err := cache.ApplyBridgeCommandLine(scanner.Bytes()); err != nil {
			t.Fatalf("line %d: %v", lineNumber, err)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}

	if _, ok := cache.SurfacePolicy("view-42"); ok {
		t.Fatal("policy_remove should delete view-42")
	}
	if _, ok := cache.ActorUID(); ok {
		t.Fatal("second input_context should clear actor uid")
	}
}

func TestCacheTracksActorUID(t *testing.T) {
	cache := NewCache()
	err := cache.ApplyBridgeCommandLine([]byte(`{"type":"input_context","actor_uid":60002}`))
	if err != nil {
		t.Fatal(err)
	}

	actorUID, ok := cache.ActorUID()
	if !ok {
		t.Fatal("actor uid was not set")
	}
	if actorUID != 60002 {
		t.Fatalf("actor uid = %d, want 60002", actorUID)
	}
}

func TestCacheRejectsCloseCommands(t *testing.T) {
	cache := NewCache()
	err := cache.ApplyBridgeCommandLine([]byte(`{"type":"close_surface","surface_id":"view-42"}`))
	if !errors.Is(err, ErrUnsupportedCommand) {
		t.Fatalf("close_surface error = %v, want ErrUnsupportedCommand", err)
	}
}

func fixturePath(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join("..", "..", "..", "compositor", "protocol-fixtures", "wayfire", name)
}

