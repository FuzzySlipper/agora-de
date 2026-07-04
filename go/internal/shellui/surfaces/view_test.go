package surfaces

import (
	"strings"
	"testing"

	"agora-de.local/go/internal/surfacetrack"
)

func TestLifecycleViewsProjectSurfaceState(t *testing.T) {
	events, err := surfacetrack.DecodeWayfireSurfaceEvents(strings.NewReader(
		`{"type":"surface_event","event":"mapped","surface":{"id":"view-42"},"client":{"pid":42,"uid":60001,"gid":60001}}` + "\n" +
			`{"type":"surface_event","event":"focused","surface":{"id":"view-42"},"client":{"pid":42,"uid":60001,"gid":60001}}` + "\n" +
			`{"type":"surface_event","event":"input_denied","surface":{"id":"view-42"},"client":{"pid":42,"uid":60001,"gid":60001}}` + "\n",
	))
	if err != nil {
		t.Fatal(err)
	}
	projection := surfacetrack.NewProjection()
	for _, event := range events {
		if err := projection.ApplyWayfireEvent(event); err != nil {
			t.Fatal(err)
		}
	}

	views := LifecycleViews(projection)
	if len(views) != 1 {
		t.Fatalf("views = %d, want 1", len(views))
	}
	view := views[0]
	if view.ID != "view-42" {
		t.Fatalf("id = %q, want view-42", view.ID)
	}
	if view.OwnerUID != 60001 || !view.Mapped || !view.Focused || view.InputDeniedCount != 1 {
		t.Fatalf("unexpected surface view: %+v", view)
	}
}

func TestLifecycleViewsAreSorted(t *testing.T) {
	events, err := surfacetrack.DecodeWayfireSurfaceEvents(strings.NewReader(
		`{"type":"surface_event","event":"mapped","surface":{"id":"view-b"},"client":{"pid":2,"uid":60002,"gid":60002}}` + "\n" +
			`{"type":"surface_event","event":"mapped","surface":{"id":"view-a"},"client":{"pid":1,"uid":60001,"gid":60001}}` + "\n",
	))
	if err != nil {
		t.Fatal(err)
	}
	projection := surfacetrack.NewProjection()
	for _, event := range events {
		if err := projection.ApplyWayfireEvent(event); err != nil {
			t.Fatal(err)
		}
	}

	views := LifecycleViews(projection)
	if len(views) != 2 {
		t.Fatalf("views = %d, want 2", len(views))
	}
	if views[0].ID != "view-a" || views[1].ID != "view-b" {
		t.Fatalf("views not sorted by id: %+v", views)
	}
}

func TestLifecycleViewsTreatNilProjectionAsEmpty(t *testing.T) {
	views := LifecycleViews(nil)
	if len(views) != 0 {
		t.Fatalf("views = %d, want 0", len(views))
	}
}
