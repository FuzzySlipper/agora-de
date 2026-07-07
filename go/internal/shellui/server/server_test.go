package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"

	"agora-de.local/go/internal/shellui/catalog"
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
		`class="panel taskbar"`,
		`class="taskbar-start"`,
		`class="taskbar-tasks running"`,
		`id="workspace-list"`,
		`class="taskbar-tray"`,
		`id="apps-button"`,
		`aria-pressed="false"`,
		`Start`,
		`id="refresh-button"`,
		`aria-label="Refresh"`,
		`id="operator-button"`,
		`aria-label="Status"`,
		`class="taskbar-button-icon"`,
		`class="visually-hidden">Refresh</span>`,
		`--taskbar-control-height`,
		`--agora-panel-control-height`,
		`var(--agora-panel-bg)`,
		`var(--agora-panel-shadow)`,
		`var(--agora-popup-shadow)`,
		`var(--agora-taskbar-minimized-bg)`,
		`var(--agora-taskbar-minimized-border)`,
		`id="running-list"`,
		`id="wm-menu"`,
		`id="wm-controls"`,
		`id="target-label"`,
		`id="focus-prev-button"`,
		`id="focus-next-button"`,
		`id="promote-button"`,
		`id="move-zone-button"`,
		`id="float-button"`,
		`id="fullscreen-button"`,
		`id="maximize-button"`,
		`id="minimize-button"`,
		`unsupportedSurfaceActions`,
		`id="close-focus-button"`,
		`id="reset-layout-button"`,
		`id="rule-button"`,
		`id="master-count-button"`,
		`id="master-ratio-button"`,
		`id="gaps-button"`,
		`id="smart-gaps-button"`,
		`id="layout-rule-label"`,
		`id="workspace-label"`,
		`id="layout-mode-button"`,
		`id="status-label"`,
		`id="clock-label"`,
		`setAttribute("aria-pressed"`,
		`io.agorade.ShellLauncher`,
		`backend_unsupported`,
		`/api/catalog/apps`,
		`/api/catalog/launch`,
		`/api/surfaces`,
		`/api/surfaces/action`,
		`/api/layout`,
		`/api/layout/action`,
		`/api/workspaces`,
		`/api/workspaces/action`,
		`assignZone`,
		`setMode`,
		`allWorkspaces`,
		`activeWorkspaceId`,
		`workspaceState`,
		`currentOutputId`,
		`workspaceOutputShortName`,
		`button.dataset.outputId`,
		`body.outputId`,
		`workspaceSurfaceCount`,
		`renderWorkspaces`,
		`task-button`,
		`task-button.minimized`,
		`content: "min"`,
		`activateTaskSurface`,
		`click to restore`,
		`workspaceZones`,
		`nextLayoutMode`,
		`nextLayoutRule`,
		`layoutSettingsLabel`,
		`normalizedSettings`,
		`manageableLayoutSurfaces`,
		`isTaskbarWorkSurface`,
		`isShellManagedSurface`,
		`isTransientSurfaceRole`,
		`state.surfaces.filter(isTaskbarWorkSurface)`,
		`renderWMControls`,
		`focusRelative`,
		`promoteTarget`,
		`moveTargetToNextZone`,
		`toggleTargetFloating`,
		`resetLayout`,
		`setSettings`,
		`actionStatus`,
		`setFeedback`,
		`floating, action: "setFloating"`,
		`surfaceAreaLabel`,
		`className = "dock-item" + (className ? " " + className : "")`,
		`surface-meta`,
		`shell-status`,
		`workspace 1`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("shell body missing %q: %s", want, body)
		}
	}
	for _, notWant := range []string{`id="apps-section"`, `id="apps-list"`, `id="app-search"`} {
		if strings.Contains(body, notWant) {
			t.Fatalf("panel body should not include horizontal app list hook %q: %s", notWant, body)
		}
	}

	launcher := responseBody(t, handler, "/shell/dist/desktop/?surface=launcher")
	for _, want := range []string{
		"agora-de app launcher",
		`class="launcher"`,
		`id="app-search"`,
		`id="categories"`,
		`id="app-list"`,
		`id="close-button"`,
		`const icon = createIcon(`,
		`icon-load-failed`,
		`className = "app-detail"`,
		`id="policy-status"`,
		`dataset.disabledCode`,
		`height: 100vh`,
		`inset: 0`,
		`width: 100vw`,
		`overflow-y: auto`,
		`launchable / " + disabled + " disabled`,
		`/api/catalog/launch`,
		`io.agorade.ShellLauncher`,
		`/api/catalog/apps`,
		`/api/surfaces/action`,
		`await closeLauncher()`,
	} {
		if !strings.Contains(launcher, want) {
			t.Fatalf("launcher body missing %q: %s", want, launcher)
		}
	}

	operator := responseBody(t, handler, "/shell/dist/desktop/?surface=operator")
	for _, want := range []string{
		"agora-de shell status",
		`id="overall"`,
		`status: loading`,
		`status: " + (status.overall || "unknown")`,
		`id="close-button"`,
		`/api/operator/status`,
		`/api/surfaces/action`,
		`io.agorade.ShellStatus`,
		`closeStatus`,
		`height: 100%`,
		`overflow: hidden`,
		"Recovery",
	} {
		if !strings.Contains(operator, want) {
			t.Fatalf("operator body missing %q: %s", want, operator)
		}
	}

	overlay := responseBody(t, handler, "/shell/dist/desktop/?surface=overlay")
	for _, want := range []string{
		"agora-de agent overlay",
		`data-surface="overlay"`,
		`id="agent-overlay-surface"`,
		`var(--agora-overlay-label-bg)`,
		`var(--agora-overlay-chip-bg)`,
		`var(--agora-focus-glow)`,
		`id="overlay-labels"`,
		`id="zone-hints"`,
		`class="zone-hint">primary`,
		`class="zone-hint">secondary`,
		`className = "window-box"`,
		`className = "number"`,
		`className = "bounds"`,
		`className = "meta"`,
		`action-hints`,
		`right: 8px`,
		`dataset.layoutRule`,
		`layoutRuleLabel`,
		`surface.focused ? " focused"`,
		`geometry.x + "," + geometry.y`,
		`/api/layout`,
		`/api/surfaces`,
	} {
		if !strings.Contains(overlay, want) {
			t.Fatalf("overlay body missing %q: %s", want, overlay)
		}
	}
	if strings.Contains(overlay, `class="panel"`) || strings.Contains(overlay, "agora-de shell: overlay") {
		t.Fatalf("overlay body = %q, should not use panel or background fallback content", overlay)
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
			Code       string `json:"disabledCode"`
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

	var layoutResponse struct {
		Layout struct {
			Mode     string `json:"mode"`
			Surfaces []struct {
				SurfaceID   string `json:"surfaceId"`
				Label       string `json:"label"`
				WorkspaceID string `json:"workspaceId"`
				ZoneID      string `json:"zoneId"`
				Focused     bool   `json:"focused"`
			} `json:"surfaces"`
			Workspaces []struct {
				ID           string   `json:"id"`
				SurfaceOrder []string `json:"surfaceOrder"`
			} `json:"workspaces"`
		} `json:"layout"`
	}
	decodeRoute(t, handler, LayoutPath, &layoutResponse)
	if layoutResponse.Layout.Mode != "freeform" || len(layoutResponse.Layout.Surfaces) != 1 {
		t.Fatalf("unexpected layout response: %+v", layoutResponse)
	}
	if layoutResponse.Layout.Surfaces[0].SurfaceID != "view-42" || layoutResponse.Layout.Surfaces[0].Label != "1" || layoutResponse.Layout.Surfaces[0].WorkspaceID != "workspace-1" || layoutResponse.Layout.Surfaces[0].ZoneID != "primary" || !layoutResponse.Layout.Surfaces[0].Focused {
		t.Fatalf("unexpected layout surface: %+v", layoutResponse.Layout.Surfaces[0])
	}
	if len(layoutResponse.Layout.Workspaces) != 1 || layoutResponse.Layout.Workspaces[0].ID != "workspace-1" || len(layoutResponse.Layout.Workspaces[0].SurfaceOrder) != 1 {
		t.Fatalf("unexpected layout workspace: %+v", layoutResponse.Layout.Workspaces)
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
		Timing struct {
			Schema string            `json:"schema"`
			Routes []routeTimingView `json:"routes"`
		} `json:"timing"`
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
	if operatorResponse.Timing.Schema != "agora-de.shell-timing.v1" || len(operatorResponse.Timing.Routes) == 0 {
		t.Fatalf("unexpected operator timing summary: %+v", operatorResponse.Timing)
	}
	if !timingRoutesContain(operatorResponse.Timing.Routes, "GET /api/catalog/apps", "shell_http", "catalog") {
		t.Fatalf("operator timing summary missing catalog route: %+v", operatorResponse.Timing.Routes)
	}
	if operatorResponse.Recovery.KillAllCommand != "sudo /usr/local/sbin/agora-de-kill-all" {
		t.Fatalf("unexpected recovery command: %+v", operatorResponse.Recovery)
	}
	if len(operatorResponse.Recovery.RestartCommands) == 0 || !strings.Contains(operatorResponse.Recovery.Runbook, "den-k8-visible-shell-runbook.md") {
		t.Fatalf("unexpected recovery docs: %+v", operatorResponse.Recovery)
	}

	var timingResponse struct {
		Schema string            `json:"schema"`
		Routes []routeTimingView `json:"routes"`
	}
	decodeRoute(t, handler, TimingDiagnosticsPath, &timingResponse)
	if timingResponse.Schema != "agora-de.shell-timing.v1" {
		t.Fatalf("unexpected timing diagnostics schema: %+v", timingResponse)
	}
	if !timingRoutesContain(timingResponse.Routes, "GET /api/operator/status", "shell_http", "operator") {
		t.Fatalf("timing diagnostics missing operator route: %+v", timingResponse.Routes)
	}
}

type routeTimingView struct {
	Name      string  `json:"name"`
	Category  string  `json:"category"`
	Backend   string  `json:"backend"`
	Count     uint64  `json:"count"`
	AverageMs float64 `json:"averageMs"`
	P95Ms     float64 `json:"p95Ms"`
}

func timingRoutesContain(routes []routeTimingView, name string, category string, backend string) bool {
	for _, route := range routes {
		if route.Name == name && route.Category == category && route.Backend == backend && route.Count > 0 {
			return true
		}
	}
	return false
}

func TestHandlerUsesSelectedTheme(t *testing.T) {
	handler, err := NewHandler(Config{
		FixtureProviders: true,
		ThemeID:          "agora-ember",
	})
	if err != nil {
		t.Fatal(err)
	}
	body := responseBody(t, handler, "/shell/dist/desktop/?surface=dock")
	if !strings.Contains(body, "--agora-accent: #fb923c;") {
		t.Fatalf("shell body missing selected theme accent: %s", body)
	}
	if !strings.Contains(body, "--agora-bg: #12100f;") {
		t.Fatalf("shell body missing selected theme background: %s", body)
	}

	var route themeResponse
	decodeRoute(t, handler, ThemePath, &route)
	if route.ID != "agora-ember" || route.Fallback {
		t.Fatalf("theme route = %+v, want ember without fallback", route)
	}
}

func TestHandlerFallsBackForInvalidThemeSelection(t *testing.T) {
	handler, err := NewHandler(Config{
		FixtureProviders: true,
		ThemeID:          "missing-theme",
	})
	if err != nil {
		t.Fatal(err)
	}
	var route themeResponse
	decodeRoute(t, handler, ThemePath, &route)
	if route.ID != "agora-default" || !route.Fallback {
		t.Fatalf("theme route = %+v, want default fallback", route)
	}
	if !strings.Contains(route.FallbackReason, "unknown bundled theme") {
		t.Fatalf("fallback reason = %q", route.FallbackReason)
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

func TestHandlerExposesLayoutViaCompositorctl(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script fixture is Unix-specific")
	}
	dir := t.TempDir()
	logPath := filepath.Join(dir, "calls.log")
	command := filepath.Join(dir, "compositorctl-fixture")
	script := `#!/usr/bin/env sh
printf '%s\n' "$*" >> "$CALL_LOG"
case "$1 $2" in
  "layout get")
    printf '%s\n' '{"layout":{"mode":"zones","revision":7,"settings":{"rule":"master_stack","mode":"zones","gaps":{"outer_horizontal":4,"outer_vertical":2,"inner_horizontal":8,"inner_vertical":6},"master_count":2,"master_ratio":0.6,"smart_gaps":true},"surfaces":[{"surface_id":"view-live","label":"1","app_id":"foot","title":"foot","output_id":"HDMI-A-1","workspace_id":"workspace-1","zone_id":"primary","mode":"zones","participation":"tiled","focused":true,"visible":true,"geometry":{"x":1,"y":2,"width":300,"height":200},"order":0}],"workspaces":[{"id":"workspace-1","name":"workspace 1","output_id":"HDMI-A-1","active":true,"zones":[{"id":"primary","name":"Primary","kind":"work","surface_ids":["view-live"]}],"surface_order":["view-live"]}]}}'
    ;;
  "layout set-mode")
    printf '%s\n' '{"decision":"accepted"}'
    ;;
  "layout set-settings")
    printf '%s\n' '{"decision":"accepted"}'
    ;;
  "surface assign-zone")
    printf '%s\n' '{"decision":"accepted"}'
    ;;
  "surface promote")
    printf '%s\n' '{"decision":"accepted"}'
    ;;
  *)
    printf 'unexpected command %s %s\n' "$1" "$2" >&2
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

	var layoutResponse struct {
		Layout struct {
			Mode     string `json:"mode"`
			Revision uint64 `json:"revision"`
			Surfaces []struct {
				SurfaceID string `json:"surfaceId"`
				AppID     string `json:"appId"`
				ZoneID    string `json:"zoneId"`
				Geometry  struct {
					Width int `json:"width"`
				} `json:"geometry"`
			} `json:"surfaces"`
			Settings struct {
				Rule        string  `json:"rule"`
				MasterCount int     `json:"masterCount"`
				MasterRatio float64 `json:"masterRatio"`
				Gaps        struct {
					InnerHorizontal int `json:"innerHorizontal"`
				} `json:"gaps"`
			} `json:"settings"`
			Workspaces []struct {
				ID           string   `json:"id"`
				SurfaceOrder []string `json:"surfaceOrder"`
			} `json:"workspaces"`
		} `json:"layout"`
	}
	decodeRoute(t, handler, LayoutPath, &layoutResponse)
	if layoutResponse.Layout.Mode != "zones" || layoutResponse.Layout.Revision != 7 || len(layoutResponse.Layout.Surfaces) != 1 {
		t.Fatalf("unexpected layout response: %+v", layoutResponse)
	}
	if layoutResponse.Layout.Surfaces[0].SurfaceID != "view-live" || layoutResponse.Layout.Surfaces[0].AppID != "foot" || layoutResponse.Layout.Surfaces[0].ZoneID != "primary" || layoutResponse.Layout.Surfaces[0].Geometry.Width != 300 {
		t.Fatalf("unexpected layout surface projection: %+v", layoutResponse.Layout.Surfaces[0])
	}
	if layoutResponse.Layout.Settings.Rule != "master_stack" || layoutResponse.Layout.Settings.MasterCount != 2 || layoutResponse.Layout.Settings.MasterRatio != 0.6 || layoutResponse.Layout.Settings.Gaps.InnerHorizontal != 8 {
		t.Fatalf("unexpected layout settings projection: %+v", layoutResponse.Layout.Settings)
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, LayoutActionPath, strings.NewReader(`{"action":"setMode","mode":"zones"}`)))
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("layout setMode status = %d, want %d; body=%s", recorder.Code, http.StatusAccepted, recorder.Body.String())
	}

	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, LayoutActionPath, strings.NewReader(`{"action":"assignZone","surfaceId":"view-live","workspaceId":"workspace-1","zoneId":"secondary","geometry":{"x":20,"y":40,"width":500,"height":360}}`)))
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("layout assignZone status = %d, want %d; body=%s", recorder.Code, http.StatusAccepted, recorder.Body.String())
	}

	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, LayoutActionPath, strings.NewReader(`{"action":"promote","surfaceId":"view-live"}`)))
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("layout promote status = %d, want %d; body=%s", recorder.Code, http.StatusAccepted, recorder.Body.String())
	}

	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, LayoutActionPath, strings.NewReader(`{"action":"setSettings","settings":{"rule":"dwindle","mode":"columns","gaps":{"outerHorizontal":4,"outerVertical":6,"innerHorizontal":8,"innerVertical":10},"masterCount":2,"masterRatio":0.6,"smartGaps":false}}`)))
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("layout setSettings status = %d, want %d; body=%s", recorder.Code, http.StatusAccepted, recorder.Body.String())
	}

	calls, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"layout get",
		"layout set-mode --mode zones",
		"layout set-settings --rule dwindle --mode columns",
		"--outer-horizontal 4 --outer-vertical 6 --inner-horizontal 8 --inner-vertical 10",
		"--master-count 2 --master-ratio 0.60 --smart-gaps=false",
		"surface assign-zone --surface view-live --zone secondary",
		"--workspace workspace-1",
		"--x 20 --y 40 --width 500 --height 360",
		"surface promote --surface view-live",
	} {
		if !strings.Contains(string(calls), want) {
			t.Fatalf("compositorctl calls missing %q: %s", want, calls)
		}
	}
}

func TestHandlerExposesWorkspaceStateViaCompositorctl(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script fixture is Unix-specific")
	}
	dir := t.TempDir()
	logPath := filepath.Join(dir, "calls.log")
	activePath := filepath.Join(dir, "active-workspace")
	command := filepath.Join(dir, "compositorctl-fixture")
	script := `#!/usr/bin/env sh
printf '%s\n' "$*" >> "$CALL_LOG"
[ -s "$ACTIVE_WORKSPACE" ] || printf '%s\n' 'workspace-1' > "$ACTIVE_WORKSPACE"
case "$1 $2" in
  "layout get")
    current="$(cat "$ACTIVE_WORKSPACE")"
    if [ "$current" = "workspace-2" ]; then
      printf '%s\n' '{"layout":{"surfaces":[{"surface_id":"view-a","output_id":"HDMI-A-1","workspace_id":"workspace-1","visible":false},{"surface_id":"view-b","output_id":"DP-1","workspace_id":"workspace-2","visible":true}],"workspaces":[{"id":"workspace-1","name":"workspace 1","output_id":"HDMI-A-1","active":false,"surface_order":["view-a"]},{"id":"workspace-2","name":"workspace 2","output_id":"DP-1","active":true,"surface_order":["view-b"]}]}}'
    else
      printf '%s\n' '{"layout":{"surfaces":[{"surface_id":"view-a","output_id":"HDMI-A-1","workspace_id":"workspace-1","visible":true},{"surface_id":"view-b","output_id":"DP-1","workspace_id":"workspace-2","visible":false}],"workspaces":[{"id":"workspace-1","name":"workspace 1","output_id":"HDMI-A-1","active":true,"surface_order":["view-a"]},{"id":"workspace-2","name":"workspace 2","output_id":"DP-1","active":false,"surface_order":["view-b"]}]}}'
    fi
    ;;
  "workspace activate")
    workspace=""
    while [ "$#" -gt 0 ]; do
      if [ "$1" = "--workspace" ]; then
        shift
        workspace="$1"
      fi
      shift || true
    done
    printf '%s\n' "$workspace" > "$ACTIVE_WORKSPACE"
    printf '%s\n' '{"decision":"accepted"}'
    ;;
  "list-surfaces")
    printf '%s\n' '{"surfaces":[]}'
    ;;
  *)
    printf 'unexpected command %s %s\n' "$1" "$2" >&2
    exit 2
    ;;
esac
`
	if err := os.WriteFile(command, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CALL_LOG", logPath)
	t.Setenv("ACTIVE_WORKSPACE", activePath)

	handler, err := NewHandler(Config{
		FixtureProviders:  true,
		SurfaceProvider:   SurfaceProviderCompositorctl,
		CompositorctlPath: command,
	})
	if err != nil {
		t.Fatal(err)
	}

	var state workspacesResponse
	decodeRoute(t, handler, WorkspacesPath, &state)
	if state.CurrentWorkspaceID != "workspace-1" || state.CurrentOutputID != "HDMI-A-1" || len(state.Workspaces) != 2 || !state.Workspaces[0].Active || state.Workspaces[1].SurfaceCount != 1 || state.Workspaces[1].OutputID != "DP-1" {
		t.Fatalf("workspace state = %+v", state)
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, WorkspaceActionPath, strings.NewReader(`{"workspaceId":"workspace-2","outputId":"DP-1","action":"activate"}`)))
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("workspace activate status = %d, want %d; body=%s", recorder.Code, http.StatusAccepted, recorder.Body.String())
	}
	var action workspaceActionResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &action); err != nil {
		t.Fatal(err)
	}
	if action.CurrentWorkspaceID != "workspace-2" || action.CurrentOutputID != "DP-1" || action.Workspace.OutputID != "DP-1" || !action.Workspace.Active || action.Workspace.SurfaceCount != 1 {
		t.Fatalf("workspace action = %+v", action)
	}
	calls, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(calls), "workspace activate --workspace workspace-2 --output DP-1 --timeout-ms 2000") {
		t.Fatalf("workspace activation call missing: %s", calls)
	}
}

func TestHandlerReturnsClassifiedLayoutActionErrors(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script fixture is Unix-specific")
	}
	command := filepath.Join(t.TempDir(), "compositorctl-fixture")
	script := `#!/usr/bin/env sh
printf '%s\n' 'server[backend_unsupported]: surface.tile requires compositor backend geometry authority' >&2
exit 1
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

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, LayoutActionPath, strings.NewReader(`{"action":"tile","surfaceId":"view-live","zoneId":"primary"}`)))
	if recorder.Code != http.StatusNotImplemented {
		t.Fatalf("layout tile status = %d, want %d; body=%s", recorder.Code, http.StatusNotImplemented, recorder.Body.String())
	}
	var body struct {
		Error      string `json:"error"`
		ErrorClass string `json:"errorClass"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.ErrorClass != "backend_unsupported" || !strings.Contains(body.Error, "surface.tile") {
		t.Fatalf("unexpected classified error: %+v", body)
	}
}

func TestParseCompositorctlErrorHandlesCliErrorPrefix(t *testing.T) {
	errorClass, message := parseCompositorctlError("error: server[backend_unsupported]: surface.fullscreen requires compositor backend geometry authority")
	if errorClass != "backend_unsupported" {
		t.Fatalf("errorClass = %q, want backend_unsupported", errorClass)
	}
	if !strings.Contains(message, "surface.fullscreen") {
		t.Fatalf("message = %q, want fullscreen detail", message)
	}
}

func TestHandlerCanUseDesktopEntryCatalogProvider(t *testing.T) {
	root := t.TempDir()
	pixmaps := t.TempDir()
	if err := os.WriteFile(filepath.Join(pixmaps, "terminal.svg"), []byte("<svg/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeServerDesktopEntry(t, root, "terminal.desktop", `[Desktop Entry]
Type=Application
Name=Terminal
Exec=terminal %U
Icon=terminal
StartupWMClass=Terminal
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
		IconPixmapRoots:   []string{pixmaps},
	})
	if err != nil {
		t.Fatal(err)
	}

	var response struct {
		Apps []struct {
			ID         string `json:"id"`
			Name       string `json:"name"`
			Icon       string `json:"icon"`
			IconURL    string `json:"iconUrl"`
			WMClass    string `json:"startupWMClass"`
			Launchable bool   `json:"launchable"`
			Code       string `json:"disabledCode"`
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
	if app.WMClass != "Terminal" {
		t.Fatalf("startup wm class = %q, want Terminal", app.WMClass)
	}
	if !strings.HasPrefix(app.IconURL, CatalogIconPathPrefix) {
		t.Fatalf("icon url = %q, want catalog icon route", app.IconURL)
	}
	iconRecorder := httptest.NewRecorder()
	handler.ServeHTTP(iconRecorder, httptest.NewRequest(http.MethodGet, app.IconURL, nil))
	if iconRecorder.Code != http.StatusOK || !strings.Contains(iconRecorder.Body.String(), "<svg/>") {
		t.Fatalf("icon response = status %d body %q", iconRecorder.Code, iconRecorder.Body.String())
	}
	if app.Launchable {
		t.Fatalf("imported native app should not be launchable without explicit target: %+v", app)
	}
	if app.Reason != "native launch disabled" {
		t.Fatalf("disabled reason = %q, want native launch disabled", app.Reason)
	}
	if app.Code != NativeDisabledCodeProviderDisabled {
		t.Fatalf("disabled code = %q, want %s", app.Code, NativeDisabledCodeProviderDisabled)
	}
}

func TestHandlerRejectsUnknownNativeLaunchProvider(t *testing.T) {
	_, err := NewHandler(Config{
		FixtureProviders:     true,
		NativeLaunchProvider: "shell",
	})
	if err == nil {
		t.Fatal("NewHandler accepted unknown native launch provider")
	}
	if !strings.Contains(err.Error(), `unknown native launch provider "shell"`) {
		t.Fatalf("error = %v, want unknown native launch provider", err)
	}
}

func TestHandlerMarksNativeAppDisabledWhenNotAllowlisted(t *testing.T) {
	root := t.TempDir()
	writeServerDesktopEntry(t, root, "terminal.desktop", `[Desktop Entry]
Type=Application
Name=Terminal
Exec=terminal --title %c
Icon=terminal
`)

	handler, err := NewHandler(Config{
		FixtureProviders:      true,
		CatalogProvider:       CatalogProviderDesktopEntries,
		DesktopEntryRoots:     []string{root},
		NativeLaunchProvider:  NativeLaunchProviderStructuredCompositorctl,
		NativeLaunchAllowlist: []string{"editor.desktop"},
	})
	if err != nil {
		t.Fatal(err)
	}

	var response struct {
		Apps []struct {
			ID         string `json:"id"`
			Launchable bool   `json:"launchable"`
			Code       string `json:"disabledCode"`
			Reason     string `json:"disabledReason"`
		} `json:"apps"`
	}
	decodeRoute(t, handler, "/api/catalog/apps", &response)
	if len(response.Apps) != 1 {
		t.Fatalf("apps = %d, want 1: %+v", len(response.Apps), response.Apps)
	}
	app := response.Apps[0]
	if app.ID != "terminal.desktop" || app.Launchable {
		t.Fatalf("native catalog app should be visible but disabled: %+v", app)
	}
	if app.Code != NativeDisabledCodeNotAllowlisted || app.Reason != "not enabled for native launch" {
		t.Fatalf("disabled state = (%q, %q), want not allowlisted", app.Code, app.Reason)
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
	for _, want := range []string{"launch", "surface=operator", "agora-de-gtk4-layer-shell-webview", "--arg --role --arg popup", "--expected-app-id io.agorade.ShellStatus"} {
		if !strings.Contains(string(calls), want) {
			t.Fatalf("status launch compositorctl calls missing %q: %s", want, calls)
		}
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/catalog/launch", strings.NewReader(`{"appId":"shell-launcher"}`))
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("launcher launch status = %d, want %d; body=%s", recorder.Code, http.StatusAccepted, recorder.Body.String())
	}
	calls, err = os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"surface=launcher", "agora-de-gtk4-layer-shell-webview", "--arg --role --arg popup", "--expected-app-id io.agorade.ShellLauncher"} {
		if !strings.Contains(string(calls), want) {
			t.Fatalf("launcher compositorctl calls missing %q: %s", want, calls)
		}
	}
}

func TestHandlerLaunchesNativeAppsWithAllowAllWildcard(t *testing.T) {
	root := t.TempDir()
	writeServerDesktopEntry(t, root, "terminal.desktop", `[Desktop Entry]
Type=Application
Name=Terminal
Exec=terminal --title %c
Icon=terminal
`)
	writeServerDesktopEntry(t, root, "browser.desktop", `[Desktop Entry]
Type=Application
Name=Browser
Exec=browser %Z
Icon=browser
`)

	handler, err := NewHandler(Config{
		FixtureProviders:      true,
		CatalogProvider:       CatalogProviderDesktopEntries,
		DesktopEntryRoots:     []string{root},
		NativeLaunchProvider:  NativeLaunchProviderStructuredCompositorctl,
		NativeLaunchAllowlist: []string{NativeLaunchAllowAll},
	})
	if err != nil {
		t.Fatal(err)
	}

	var response struct {
		Apps []struct {
			ID         string `json:"id"`
			Launchable bool   `json:"launchable"`
			Code       string `json:"disabledCode"`
			Reason     string `json:"disabledReason"`
		} `json:"apps"`
	}
	decodeRoute(t, handler, "/api/catalog/apps", &response)
	apps := map[string]struct {
		Launchable bool
		Code       string
		Reason     string
	}{}
	for _, app := range response.Apps {
		apps[app.ID] = struct {
			Launchable bool
			Code       string
			Reason     string
		}{Launchable: app.Launchable, Code: app.Code, Reason: app.Reason}
	}
	if !apps["terminal.desktop"].Launchable || apps["terminal.desktop"].Code != "" || apps["terminal.desktop"].Reason != "" {
		t.Fatalf("wildcard should make preparable app launchable: %+v", apps["terminal.desktop"])
	}
	if apps["browser.desktop"].Launchable || apps["browser.desktop"].Code != catalog.DisabledCodeUnsupportedDesktopEntry {
		t.Fatalf("wildcard should not make unsupported desktop entry launchable: %+v", apps["browser.desktop"])
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
			Code       string `json:"disabledCode"`
			Reason     string `json:"disabledReason"`
		} `json:"apps"`
	}
	decodeRoute(t, handler, "/api/catalog/apps", &catalogResponse)
	if len(catalogResponse.Apps) != 1 || catalogResponse.Apps[0].ID != "terminal.desktop" || !catalogResponse.Apps[0].Launchable {
		t.Fatalf("native catalog app not launchable through structured provider: %+v", catalogResponse.Apps)
	}
	if catalogResponse.Apps[0].Code != "" || catalogResponse.Apps[0].Reason != "" {
		t.Fatalf("allowlisted native app disabled state = (%q, %q), want empty", catalogResponse.Apps[0].Code, catalogResponse.Apps[0].Reason)
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
		"--expected-app-id",
		"terminal",
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
	for _, want := range []string{"surface=operator", "agora-de-gtk4-layer-shell-webview", "--arg --role --arg popup", "--expected-app-id io.agorade.ShellStatus"} {
		if !strings.Contains(string(calls), want) {
			t.Fatalf("status launch compositorctl calls missing %q: %s", want, calls)
		}
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/catalog/launch", strings.NewReader(`{"appId":"shell-launcher"}`))
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("launcher launch status = %d, want %d; body=%s", recorder.Code, http.StatusAccepted, recorder.Body.String())
	}
	calls, err = os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"surface=launcher", "agora-de-gtk4-layer-shell-webview", "--arg --role --arg popup", "--expected-app-id io.agorade.ShellLauncher"} {
		if !strings.Contains(string(calls), want) {
			t.Fatalf("launcher compositorctl calls missing %q: %s", want, calls)
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

func TestHandlerClosesShellLayerByTerminatingShellClient(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script fixture is Unix-specific")
	}
	dir := t.TempDir()
	logPath := filepath.Join(dir, "calls.log")
	command := filepath.Join(dir, "compositorctl-fixture")
	script := `#!/usr/bin/env sh
printf '%s\n' "$*" >> "$CALL_LOG"
case "$1" in
  list-surfaces)
    printf '%s\n' '{"surfaces":[{"surface":{"id":"layer-status","app_id":"io.agorade.ShellStatus","surface_kind":"layer_shell","visible":true},"client":{"pid":424242,"uid":60010},"last_event":"content_committed","visible":true}]}'
    ;;
  surface)
    printf '%s\n' '{"status":"accepted"}'
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
	var signaledPID int
	var signaledSignal syscall.Signal
	originalSignalProcess := signalProcess
	signalProcess = func(pid int, signal syscall.Signal) error {
		signaledPID = pid
		signaledSignal = signal
		return nil
	}
	t.Cleanup(func() { signalProcess = originalSignalProcess })

	handler, err := NewHandler(Config{
		FixtureProviders:  true,
		SurfaceProvider:   SurfaceProviderCompositorctl,
		CompositorctlPath: command,
	})
	if err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	body := strings.NewReader(`{"surfaceId":"layer-status","action":"close"}`)
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, SurfaceActionPath, body))
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("close launcher status = %d, want %d; body=%s", recorder.Code, http.StatusAccepted, recorder.Body.String())
	}
	if signaledPID != 424242 || signaledSignal != syscall.SIGTERM {
		t.Fatalf("signal = (%d, %v), want (424242, SIGTERM)", signaledPID, signaledSignal)
	}
	calls, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(calls), "list-surfaces") {
		t.Fatalf("compositorctl calls missing list-surfaces: %s", calls)
	}
	if strings.Contains(string(calls), "surface close --surface layer-status") {
		t.Fatalf("launcher layer close should not use work-surface close: %s", calls)
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
