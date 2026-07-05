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
	assertNoStore(t, handler, "/shell/dist/desktop/?surface=dock")
	assertNoStore(t, handler, "/api/catalog/apps")
	if !strings.Contains(strings.ToLower(body), "<!doctype html>") {
		t.Fatalf("shell body = %q, want doctype html", body)
	}
	if !strings.Contains(body, "--agora-bg") || !strings.Contains(body, "var(--agora-evidence-accent)") {
		t.Fatalf("shell body = %q, want centralized theme tokens", body)
	}
	for _, want := range []string{
		`class="panel"`,
		`id="apps-button"`,
		`aria-pressed="false"`,
		`id="app-search"`,
		`id="apps-section"`,
		`id="refresh-button"`,
		`id="operator-button"`,
		`id="apps-list"`,
		`id="running-list"`,
		`id="workspace-label"`,
		`id="status-label"`,
		`id="clock-label"`,
		`className = "app-icon"`,
		`className = "app-meta"`,
		`apps-open`,
		`Hide Apps`,
		`setAttribute("aria-pressed"`,
		`/api/catalog/apps`,
		`/api/catalog/launch`,
		`/api/surfaces`,
		`/api/surfaces/action`,
		`/api/workspaces`,
		`/api/workspaces/action`,
		`shell-status`,
		`workspace 1`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("shell body missing %q: %s", want, body)
		}
	}

	operator := responseBody(t, handler, "/shell/dist/desktop/?surface=operator")
	for _, want := range []string{
		"agora-de shell status",
		`id="overall"`,
		`/api/operator/status`,
		"Recovery",
	} {
		if !strings.Contains(operator, want) {
			t.Fatalf("operator body missing %q: %s", want, operator)
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
			ID         string `json:"id"`
			Name       string `json:"name"`
			Icon       string `json:"icon"`
			IconKind   string `json:"iconKind"`
			IconRef    string `json:"iconRef"`
			IconLabel  string `json:"iconLabel"`
			Category   string `json:"category"`
			Launchable bool   `json:"launchable"`
			Reason     string `json:"disabledReason"`
		} `json:"apps"`
	}
	decodeRoute(t, handler, "/api/catalog/apps", &catalogResponse)
	if len(catalogResponse.Apps) != 2 {
		t.Fatalf("unexpected catalog response: %+v", catalogResponse)
	}
	seen := map[string]bool{}
	for _, app := range catalogResponse.Apps {
		seen[app.ID] = app.Launchable
		if app.IconKind == "" || app.IconRef == "" || app.IconLabel == "" || app.Category == "" {
			t.Fatalf("catalog app missing icon/category projection: %+v", app)
		}
	}
	for _, id := range []string{"example-browser", "shell-status"} {
		if !seen[id] {
			t.Fatalf("catalog app %q should be launchable: %+v", id, catalogResponse.Apps)
		}
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

	var workspacesResponse struct {
		CurrentWorkspaceID string `json:"currentWorkspaceId"`
		Workspaces         []struct {
			ID           string `json:"id"`
			Name         string `json:"name"`
			Active       bool   `json:"active"`
			SurfaceCount int    `json:"surfaceCount"`
		} `json:"workspaces"`
	}
	decodeRoute(t, handler, WorkspacesPath, &workspacesResponse)
	if workspacesResponse.CurrentWorkspaceID != "workspace-1" || len(workspacesResponse.Workspaces) != 1 {
		t.Fatalf("unexpected workspace response: %+v", workspacesResponse)
	}
	if !workspacesResponse.Workspaces[0].Active || workspacesResponse.Workspaces[0].SurfaceCount != 1 {
		t.Fatalf("unexpected workspace view: %+v", workspacesResponse.Workspaces[0])
	}

	recorder := httptest.NewRecorder()
	workspaceActionBody := strings.NewReader(`{"workspaceId":"workspace-1","action":"activate"}`)
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, WorkspaceActionPath, workspaceActionBody))
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("workspace action status = %d, want %d; body=%s", recorder.Code, http.StatusAccepted, recorder.Body.String())
	}
	var workspaceAction workspaceActionResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &workspaceAction); err != nil {
		t.Fatal(err)
	}
	if workspaceAction.CurrentWorkspaceID != "workspace-1" || workspaceAction.Status != "accepted" {
		t.Fatalf("unexpected workspace action response: %+v", workspaceAction)
	}

	var operatorResponse struct {
		Overall  string `json:"overall"`
		Services []struct {
			Name  string `json:"name"`
			Scope string `json:"scope"`
			State string `json:"state"`
		} `json:"services"`
		Sockets []struct {
			Path  string `json:"path"`
			State string `json:"state"`
		} `json:"sockets"`
		Surfaces struct {
			State string `json:"state"`
			Total int    `json:"total"`
		} `json:"surfaces"`
		Recovery struct {
			KillAllCommand  string   `json:"killAllCommand"`
			RestartCommands []string `json:"restartCommands"`
			Runbook         string   `json:"runbook"`
			Note            string   `json:"note"`
		} `json:"recovery"`
	}
	decodeRoute(t, handler, OperatorStatusPath, &operatorResponse)
	if operatorResponse.Overall == "" || len(operatorResponse.Services) == 0 || len(operatorResponse.Sockets) == 0 {
		t.Fatalf("unexpected operator status response: %+v", operatorResponse)
	}
	if operatorResponse.Surfaces.State != "available" || operatorResponse.Surfaces.Total != 1 {
		t.Fatalf("unexpected operator surface summary: %+v", operatorResponse.Surfaces)
	}
	if operatorResponse.Recovery.KillAllCommand != "sudo /usr/local/sbin/agora-de-kill-all" {
		t.Fatalf("unexpected recovery command: %+v", operatorResponse.Recovery)
	}
	if len(operatorResponse.Recovery.RestartCommands) == 0 || !strings.Contains(operatorResponse.Recovery.Runbook, "den-k8-visible-shell-runbook.md") {
		t.Fatalf("unexpected recovery docs: %+v", operatorResponse.Recovery)
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

func TestHandlerCanUseDesktopEntryCatalogProvider(t *testing.T) {
	root := t.TempDir()
	writeServerDesktopEntry(t, root, "terminal.desktop", `[Desktop Entry]
Type=Application
Name=Terminal
Exec=terminal %U
Icon=terminal
`)
	writeServerDesktopEntry(t, root, "hidden.desktop", `[Desktop Entry]
Type=Application
Name=Hidden
Exec=hidden
NoDisplay=true
`)

	handler, err := NewHandler(Config{
		FixtureProviders:  true,
		CatalogProvider:   CatalogProviderDesktopEntries,
		DesktopEntryRoots: []string{root},
	})
	if err != nil {
		t.Fatal(err)
	}

	var response struct {
		Apps []struct {
			ID         string `json:"id"`
			Name       string `json:"name"`
			Icon       string `json:"icon"`
			Launchable bool   `json:"launchable"`
			Reason     string `json:"disabledReason"`
		} `json:"apps"`
	}
	decodeRoute(t, handler, "/api/catalog/apps", &response)
	if len(response.Apps) != 1 {
		t.Fatalf("apps = %d, want 1: %+v", len(response.Apps), response.Apps)
	}
	app := response.Apps[0]
	if app.ID != "terminal.desktop" || app.Name != "Terminal" || app.Icon != "terminal" {
		t.Fatalf("unexpected app: %+v", app)
	}
	if app.Launchable {
		t.Fatalf("imported native app should not be launchable without explicit target: %+v", app)
	}
	if app.Reason != "native launch disabled" {
		t.Fatalf("disabled reason = %q, want native launch disabled", app.Reason)
	}
}

func TestHandlerLaunchesBuiltInStatusOutsideActiveCatalog(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script fixture is Unix-specific")
	}
	root := t.TempDir()
	writeServerDesktopEntry(t, root, "terminal.desktop", `[Desktop Entry]
Type=Application
Name=Terminal
Exec=terminal
Icon=terminal
`)

	dir := t.TempDir()
	logPath := filepath.Join(dir, "calls.log")
	command := filepath.Join(dir, "compositorctl-fixture")
	script := `#!/usr/bin/env sh
printf '%s\n' "$*" >> "$CALL_LOG"
printf '%s\n' '{"launch_id":"status-launch","surface":{"surface":{"id":"status-view"}}}'
`
	if err := os.WriteFile(command, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CALL_LOG", logPath)

	handler, err := NewHandler(Config{
		FixtureProviders:  true,
		CatalogProvider:   CatalogProviderDesktopEntries,
		DesktopEntryRoots: []string{root},
		CompositorctlPath: command,
	})
	if err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/catalog/launch", strings.NewReader(`{"appId":"shell-status"}`))
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("status launch status = %d, want %d; body=%s", recorder.Code, http.StatusAccepted, recorder.Body.String())
	}
	calls, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"launch", "surface=operator", "--expected-app-id io.agorade.ShellStatus"} {
		if !strings.Contains(string(calls), want) {
			t.Fatalf("status launch compositorctl calls missing %q: %s", want, calls)
		}
	}
}

func TestHandlerLaunchesAllowlistedNativeAppThroughStructuredProvider(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script fixture is Unix-specific")
	}
	root := t.TempDir()
	writeServerDesktopEntry(t, root, "terminal.desktop", `[Desktop Entry]
Type=Application
Name=Terminal
Exec=terminal --title %c
Icon=terminal
`)

	dir := t.TempDir()
	logPath := filepath.Join(dir, "calls.log")
	command := filepath.Join(dir, "compositorctl-fixture")
	script := `#!/usr/bin/env sh
printf '%s\n' "$@" >> "$CALL_LOG"
printf '%s\n' '{"launch_id":"native-launch","surface":{"surface":{"id":"native-view"}},"status":"launched"}'
`
	if err := os.WriteFile(command, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CALL_LOG", logPath)

	handler, err := NewHandler(Config{
		FixtureProviders:         true,
		CatalogProvider:          CatalogProviderDesktopEntries,
		DesktopEntryRoots:        []string{root},
		CompositorctlPath:        command,
		NativeLaunchProvider:     NativeLaunchProviderStructuredCompositorctl,
		NativeLaunchAllowlist:    []string{"terminal.desktop"},
		NativeLaunchRequesterUID: 1000,
		NativeLaunchRequesterGID: 1000,
		NativeLaunchSessionToken: "session-1",
		NativeLaunchOutputName:   "HDMI-A-1",
		NativeLaunchHome:         t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}

	var catalogResponse struct {
		Apps []struct {
			ID         string `json:"id"`
			Launchable bool   `json:"launchable"`
			Reason     string `json:"disabledReason"`
		} `json:"apps"`
	}
	decodeRoute(t, handler, "/api/catalog/apps", &catalogResponse)
	if len(catalogResponse.Apps) != 1 || catalogResponse.Apps[0].ID != "terminal.desktop" || !catalogResponse.Apps[0].Launchable {
		t.Fatalf("native catalog app not launchable through structured provider: %+v", catalogResponse.Apps)
	}
	if catalogResponse.Apps[0].Reason != "" {
		t.Fatalf("allowlisted native app disabled reason = %q, want empty", catalogResponse.Apps[0].Reason)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/catalog/launch", strings.NewReader(`{"appId":"terminal.desktop"}`))
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusAccepted, recorder.Body.String())
	}
	var launchResponse struct {
		AppID     string `json:"appId"`
		LaunchID  string `json:"launchId"`
		SurfaceID string `json:"surfaceId"`
		Status    string `json:"status"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &launchResponse); err != nil {
		t.Fatal(err)
	}
	if launchResponse.AppID != "terminal.desktop" || launchResponse.LaunchID != "native-launch" || launchResponse.SurfaceID != "native-view" || launchResponse.Status != "launched" {
		t.Fatalf("unexpected launch response: %+v", launchResponse)
	}

	calls, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	callText := string(calls)
	if strings.Contains(callText, "--cmd") || strings.Contains(callText, "terminal --title Terminal") {
		t.Fatalf("native launch used shell-shaped command: %s", callText)
	}
	for _, want := range []string{
		"launch",
		"--arg",
		"terminal",
		"--title",
		"Terminal",
		"--session-token",
		"session-1",
		"--audit-correlation-id",
		"shellui:terminal.desktop",
		"--output",
		"HDMI-A-1",
		"--wait-surface",
	} {
		if !strings.Contains(callText, want) {
			t.Fatalf("structured native launch missing %q: %s", want, callText)
		}
	}
}

func TestHandlerMarksDeadCompositorctlClientUnmapped(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script fixture is Unix-specific")
	}
	command := filepath.Join(t.TempDir(), "compositorctl-fixture")
	script := `#!/usr/bin/env sh
printf '%s\n' '{"surfaces":[{"surface":{"id":"layer-stale","app_id":"io.agorade.ShellPanel","surface_kind":"layer_shell","visible":true},"client":{"pid":99999999,"uid":60010},"last_event":"content_committed","visible":true,"content_commit_count":3}]}'
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
			ID     string `json:"id"`
			Mapped bool   `json:"mapped"`
		} `json:"surfaces"`
	}
	decodeRoute(t, handler, "/api/surfaces", &response)
	if len(response.Surfaces) != 1 {
		t.Fatalf("surfaces = %d, want 1", len(response.Surfaces))
	}
	if response.Surfaces[0].Mapped {
		t.Fatalf("dead client surface should not be mapped: %+v", response.Surfaces[0])
	}
}

func TestHandlerLaunchesAppThroughCompositorctl(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script fixture is Unix-specific")
	}
	dir := t.TempDir()
	logPath := filepath.Join(dir, "calls.log")
	command := filepath.Join(dir, "compositorctl-fixture")
	script := `#!/usr/bin/env sh
printf '%s\n' "$*" >> "$CALL_LOG"
case "$1" in
  launch)
    printf '%s\n' '{"launch_id":"launch-test","surface":{"surface":{"id":"view-test"}}}'
    ;;
  list-surfaces)
    printf '%s\n' '{"surfaces":[]}'
    ;;
  *)
    printf 'unexpected command %s\n' "$1" >&2
    exit 2
    ;;
esac
`
	if err := os.WriteFile(command, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CALL_LOG", logPath)

	handler, err := NewHandler(Config{
		FixtureProviders:  true,
		SurfaceProvider:   SurfaceProviderCompositorctl,
		CompositorctlPath: command,
	})
	if err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/catalog/launch", strings.NewReader(`{"appId":"example-browser"}`))
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusAccepted, recorder.Body.String())
	}
	var response struct {
		AppID     string `json:"appId"`
		LaunchID  string `json:"launchId"`
		SurfaceID string `json:"surfaceId"`
		Status    string `json:"status"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.AppID != "example-browser" || response.LaunchID != "launch-test" || response.SurfaceID != "view-test" || response.Status != "launched" {
		t.Fatalf("unexpected launch response: %+v", response)
	}
	calls, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"launch", "--url", "--expected-app-id io.agorade.ExampleBrowser", "--wait-surface"} {
		if !strings.Contains(string(calls), want) {
			t.Fatalf("compositorctl calls missing %q: %s", want, calls)
		}
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/catalog/launch", strings.NewReader(`{"appId":"shell-status"}`))
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("status launch status = %d, want %d; body=%s", recorder.Code, http.StatusAccepted, recorder.Body.String())
	}
	calls, err = os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"surface=operator", "--expected-app-id io.agorade.ShellStatus"} {
		if !strings.Contains(string(calls), want) {
			t.Fatalf("status launch compositorctl calls missing %q: %s", want, calls)
		}
	}
}

func TestHandlerRunsSurfaceActionsThroughCompositorctl(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script fixture is Unix-specific")
	}
	dir := t.TempDir()
	logPath := filepath.Join(dir, "calls.log")
	command := filepath.Join(dir, "compositorctl-fixture")
	script := `#!/usr/bin/env sh
printf '%s\n' "$*" >> "$CALL_LOG"
case "$1" in
  surface)
    printf '%s\n' '{"status":"accepted"}'
    ;;
  list-surfaces)
    printf '%s\n' '{"surfaces":[]}'
    ;;
  *)
    printf 'unexpected command %s\n' "$1" >&2
    exit 2
    ;;
esac
`
	if err := os.WriteFile(command, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CALL_LOG", logPath)

	handler, err := NewHandler(Config{
		FixtureProviders:  true,
		SurfaceProvider:   SurfaceProviderCompositorctl,
		CompositorctlPath: command,
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, action := range []string{"focus", "close"} {
		recorder := httptest.NewRecorder()
		body := strings.NewReader(`{"surfaceId":"view-test","action":"` + action + `"}`)
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, SurfaceActionPath, body))
		if recorder.Code != http.StatusAccepted {
			t.Fatalf("%s status = %d, want %d; body=%s", action, recorder.Code, http.StatusAccepted, recorder.Body.String())
		}
	}
	calls, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"surface focus --surface view-test", "surface close --surface view-test"} {
		if !strings.Contains(string(calls), want) {
			t.Fatalf("compositorctl calls missing %q: %s", want, calls)
		}
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

func assertNoStore(t *testing.T, handler http.Handler, path string) {
	t.Helper()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
	if got := recorder.Header().Get("Cache-Control"); !strings.Contains(got, "no-store") {
		t.Fatalf("%s Cache-Control = %q, want no-store", path, got)
	}
	if got := recorder.Header().Get("Pragma"); got != "no-cache" {
		t.Fatalf("%s Pragma = %q, want no-cache", path, got)
	}
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

func writeServerDesktopEntry(t *testing.T, root string, name string, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
