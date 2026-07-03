package surfacetrack

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProjectionAppliesWayfirePluginFixture(t *testing.T) {
	file, err := os.Open(fixturePath(t, "plugin-events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	events, err := DecodeWayfireSurfaceEvents(file)
	if err != nil {
		t.Fatal(err)
	}

	projection := NewProjection()
	for _, event := range events {
		if err := projection.ApplyWayfireEvent(event); err != nil {
			t.Fatal(err)
		}
	}

	surface, ok := projection.Surface("view-42")
	if !ok {
		t.Fatal("surface view-42 was not tracked")
	}
	if !surface.Mapped {
		t.Fatal("surface should remain mapped")
	}
	if !surface.Focused {
		t.Fatal("surface should be focused")
	}
	if projection.FocusedSurfaceID() != "view-42" {
		t.Fatalf("focused surface = %q, want view-42", projection.FocusedSurfaceID())
	}
	if surface.OwnerUID != 60001 {
		t.Fatalf("owner uid = %d, want 60001", surface.OwnerUID)
	}
	if surface.DeniedInputs != 1 {
		t.Fatalf("denied inputs = %d, want 1", surface.DeniedInputs)
	}
}

func TestProjectionUnmapClearsFocus(t *testing.T) {
	projection := NewProjection()
	events, err := DecodeWayfireSurfaceEvents(stringsReader(
		`{"type":"surface_event","event":"mapped","surface":{"id":"view-9"},"client":{"pid":9,"uid":60009,"gid":60009}}` + "\n" +
			`{"type":"surface_event","event":"focused","surface":{"id":"view-9"},"client":{"pid":9,"uid":60009,"gid":60009}}` + "\n" +
			`{"type":"surface_event","event":"unmapped","surface":{"id":"view-9"},"client":{"pid":9,"uid":60009,"gid":60009}}` + "\n",
	))
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range events {
		if err := projection.ApplyWayfireEvent(event); err != nil {
			t.Fatal(err)
		}
	}

	surface, ok := projection.Surface("view-9")
	if !ok {
		t.Fatal("surface view-9 was not tracked")
	}
	if surface.Mapped {
		t.Fatal("unmapped surface should not remain mapped")
	}
	if projection.FocusedSurfaceID() != "" {
		t.Fatalf("focused surface = %q, want empty", projection.FocusedSurfaceID())
	}
}

func fixturePath(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join("..", "..", "..", "compositor", "protocol-fixtures", "wayfire", name)
}

func stringsReader(value string) *strings.Reader {
	return strings.NewReader(value)
}
