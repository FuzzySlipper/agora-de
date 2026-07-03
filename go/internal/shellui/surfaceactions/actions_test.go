package surfaceactions

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestDecodeBridgeActionLinesFromFixture(t *testing.T) {
	file, err := os.Open(fixturePath(t, "bridge-commands.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	var actions []Action
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		action, err := DecodeBridgeActionLine(line)
		if errors.Is(err, ErrUnsupportedAction) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		actions = append(actions, action)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}

	want := []Action{
		{Kind: ActionCloseSurface, SurfaceID: "view-42"},
		{Kind: ActionCloseSurfacesByUID, OwnerUID: 60001},
	}
	if len(actions) != len(want) {
		t.Fatalf("decoded %d actions, want %d", len(actions), len(want))
	}
	for index, got := range actions {
		if got != want[index] {
			t.Fatalf("action %d = %#v, want %#v", index, got, want[index])
		}
	}
}

func TestDecodeBridgeActionLineRejectsPolicyCommand(t *testing.T) {
	_, err := DecodeBridgeActionLine([]byte(`{"type":"policy_remove","surface_id":"view-42"}`))
	if !errors.Is(err, ErrUnsupportedAction) {
		t.Fatalf("policy command error = %v, want ErrUnsupportedAction", err)
	}
}

func fixturePath(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join("..", "..", "..", "..", "compositor", "protocol-fixtures", "wayfire", name)
}
