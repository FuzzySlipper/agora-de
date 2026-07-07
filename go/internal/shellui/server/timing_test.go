package server

import (
	"net/http"
	"testing"
	"time"
)

func TestTimingRecorderAggregatesAndClassifiesRoutes(t *testing.T) {
	recorder := newTimingRecorder(timingConfig{UseCompositorctl: true})

	recorder.observe(http.MethodGet, "/api/layout", http.StatusOK, 10*time.Millisecond)
	recorder.observe(http.MethodGet, "/api/layout", http.StatusOK, 30*time.Millisecond)
	recorder.observe(http.MethodPost, "/api/surfaces/action", http.StatusAccepted, 50*time.Millisecond)
	recorder.observe(http.MethodPost, "/api/catalog/launch", http.StatusAccepted, 90*time.Millisecond)

	summary := recorder.summary(time.Unix(10, 0))
	if summary.Schema != "agora-de.shell-timing.v1" || summary.WindowSampleLimit != timingRecentSampleLimit {
		t.Fatalf("unexpected timing summary metadata: %+v", summary)
	}

	layout := findTimingRoute(t, summary, "GET /api/layout")
	if layout.Count != 2 || layout.Category != "shell_http" || layout.Backend != "compositorctl" {
		t.Fatalf("unexpected layout timing view: %+v", layout)
	}
	if layout.MinMs != 10 || layout.P50Ms != 20 || layout.P95Ms != 29 || layout.MaxMs != 30 {
		t.Fatalf("unexpected layout timing stats: %+v", layout)
	}
	if layout.StatusClasses["2xx"] != 2 {
		t.Fatalf("unexpected layout status classes: %+v", layout.StatusClasses)
	}

	surfaceAction := findTimingRoute(t, summary, "POST /api/surfaces/action")
	if surfaceAction.Category != "compositor_action" || surfaceAction.Backend != "compositorctl" {
		t.Fatalf("unexpected surface action classification: %+v", surfaceAction)
	}

	launch := findTimingRoute(t, summary, "POST /api/catalog/launch")
	if launch.Category != "launch_action" || launch.Backend != "native_launch" {
		t.Fatalf("unexpected launch classification: %+v", launch)
	}
}

func findTimingRoute(t *testing.T, summary timingSummaryResponse, name string) timingSummaryView {
	t.Helper()
	for _, route := range summary.Routes {
		if route.Name == name {
			return route
		}
	}
	t.Fatalf("timing route %q missing from %+v", name, summary.Routes)
	return timingSummaryView{}
}
