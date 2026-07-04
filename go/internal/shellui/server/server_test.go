package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestHandlerServesShellAndClaimRoutes(t *testing.T) {
	handler, err := NewHandler(Config{FixtureProviders: true})
	if err != nil {
		t.Fatal(err)
	}

	assertStatus(t, handler, "/shell/dist/desktop/?surface=dock", http.StatusOK)
	body := responseBody(t, handler, "/shell/dist/desktop/?surface=dock")
	if !strings.Contains(strings.ToLower(body), "<!doctype html>") {
		t.Fatalf("shell body = %q, want doctype html", body)
	}
	if !strings.Contains(body, "#f8fafc") || !strings.Contains(body, "#00d1b2") {
		t.Fatalf("shell body = %q, want high-contrast fallback paint styles", body)
	}
	for _, want := range []string{
		`class="panel"`,
		`id="apps-button"`,
		`id="refresh-button"`,
		`id="apps-list"`,
		`id="running-list"`,
		`id="workspace-label"`,
		`id="status-label"`,
		`id="clock-label"`,
		`/api/catalog/apps`,
		`/api/surfaces`,
		`workspace 1`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("shell body missing %q: %s", want, body)
		}
	}

	background := responseBody(t, handler, "/shell/dist/desktop/?surface=background")
	if strings.Contains(background, `class="panel"`) {
		t.Fatalf("background body = %q, should not use panel fallback content", background)
	}
	if !strings.Contains(background, "agora-de shell: background") {
		t.Fatalf("background body = %q, want background label", background)
	}
	if strings.Contains(background, `class="taskbar"`) || strings.Contains(background, "shell: dock") {
		t.Fatalf("background body = %q, should not include fallback taskbar by default", background)
	}

	fallback := responseBody(t, handler, "/shell/dist/desktop/?surface=background-fallback")
	if !strings.Contains(fallback, `class="taskbar"`) || !strings.Contains(fallback, "shell: dock") {
		t.Fatalf("fallback body = %q, want fallback taskbar content", fallback)
	}

	var catalogResponse struct {
		Apps []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			Icon string `json:"icon"`
		} `json:"apps"`
	}
	decodeRoute(t, handler, "/api/catalog/apps", &catalogResponse)
	if len(catalogResponse.Apps) != 1 || catalogResponse.Apps[0].ID != "example-browser" {
		t.Fatalf("unexpected catalog response: %+v", catalogResponse)
	}

	var surfacesResponse struct {
		Surfaces []struct {
			ID               string `json:"id"`
			OwnerUID         int    `json:"ownerUid"`
			Mapped           bool   `json:"mapped"`
			Focused          bool   `json:"focused"`
			InputDeniedCount int    `json:"inputDeniedCount"`
		} `json:"surfaces"`
	}
	decodeRoute(t, handler, "/api/surfaces", &surfacesResponse)
	if len(surfacesResponse.Surfaces) != 1 || !surfacesResponse.Surfaces[0].Focused {
		t.Fatalf("unexpected surfaces response: %+v", surfacesResponse)
	}

	var workControlsResponse struct {
		Surfaces []struct {
			ID string `json:"id"`
		} `json:"surfaces"`
	}
	decodeRoute(t, handler, WorkControlsPath, &workControlsResponse)
	if len(workControlsResponse.Surfaces) != 1 || workControlsResponse.Surfaces[0].ID != "view-42" {
		t.Fatalf("unexpected work controls response: %+v", workControlsResponse)
	}
}

func TestHandlerFailsWithoutProviderMode(t *testing.T) {
	_, err := NewHandler(Config{})
	if err == nil {
		t.Fatal("NewHandler accepted missing providers")
	}
}

func TestHandlerCanUseCompositorctlSurfaceProvider(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script fixture is Unix-specific")
	}
	command := filepath.Join(t.TempDir(), "compositorctl-fixture")
	script := `#!/usr/bin/env sh
printf '%s\n' '{"surfaces":[{"surface":{"id":"view-live","visible":true},"client":{"uid":60010},"last_event":"content_committed","focused":true,"frame_count":0,"content_commit_count":3}]}'
`
	if err := os.WriteFile(command, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	handler, err := NewHandler(Config{
		FixtureProviders:  true,
		SurfaceProvider:   SurfaceProviderCompositorctl,
		CompositorctlPath: command,
	})
	if err != nil {
		t.Fatal(err)
	}

	var response struct {
		Surfaces []struct {
			ID                 string `json:"id"`
			OwnerUID           int    `json:"ownerUid"`
			Mapped             bool   `json:"mapped"`
			Focused            bool   `json:"focused"`
			ContentCommitCount int    `json:"contentCommitCount"`
		} `json:"surfaces"`
	}
	decodeRoute(t, handler, "/api/surfaces", &response)
	if len(response.Surfaces) != 1 {
		t.Fatalf("surfaces = %d, want 1", len(response.Surfaces))
	}
	surface := response.Surfaces[0]
	if surface.ID != "view-live" || surface.OwnerUID != 60010 || !surface.Mapped || !surface.Focused || surface.ContentCommitCount != 3 {
		t.Fatalf("unexpected live surface response: %+v", surface)
	}
}

func TestHandlerFailsClosedWhenCompositorctlProviderFails(t *testing.T) {
	handler, err := NewHandler(Config{
		FixtureProviders:  true,
		SurfaceProvider:   SurfaceProviderCompositorctl,
		CompositorctlPath: filepath.Join(t.TempDir(), "missing-compositorctl"),
	})
	if err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/surfaces", nil))
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}
}

func assertStatus(t *testing.T, handler http.Handler, path string, want int) {
	t.Helper()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
	if recorder.Code != want {
		t.Fatalf("%s status = %d, want %d", path, recorder.Code, want)
	}
}

func responseBody(t *testing.T, handler http.Handler, path string) string {
	t.Helper()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("%s status = %d, want %d", path, recorder.Code, http.StatusOK)
	}
	return recorder.Body.String()
}

func decodeRoute(t *testing.T, handler http.Handler, path string, value any) {
	t.Helper()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("%s status = %d, want %d", path, recorder.Code, http.StatusOK)
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), value); err != nil {
		t.Fatalf("%s JSON decode: %v", path, err)
	}
}
