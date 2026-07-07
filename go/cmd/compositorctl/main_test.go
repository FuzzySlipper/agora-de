package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestRunLaunchStartsStructuredArgvWithoutShellCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script fixture is Unix-specific")
	}
	dir := t.TempDir()
	callsPath := filepath.Join(dir, "calls")
	envPath := filepath.Join(dir, "env")
	cwdPath := filepath.Join(dir, "cwd")
	launcher := filepath.Join(dir, "app")
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$@\" > " + shellQuote(callsPath) + "\n" +
		"printf '%s\\n' \"$FOO\" > " + shellQuote(envPath) + "\n" +
		"pwd > " + shellQuote(cwdPath) + "\n"
	if err := os.WriteFile(launcher, []byte(script), 0o700); err != nil {
		t.Fatalf("write launcher: %v", err)
	}

	var stdout bytes.Buffer
	err := run([]string{
		"launch",
		"--arg", launcher,
		"--arg", "two words",
		"--env", "FOO=bar",
		"--cwd", dir,
		"--session-token", "session-1",
		"--audit-correlation-id", "audit-1",
	}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("run launch error = %v", err)
	}
	var response launchResponse
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.LaunchID == "" || response.PID == 0 || response.Status != "launched_without_surface" {
		t.Fatalf("response = %+v", response)
	}
	waitForFile(t, callsPath)
	waitForFile(t, envPath)
	waitForFile(t, cwdPath)
	assertFile(t, callsPath, "two words\n")
	assertFile(t, envPath, "bar\n")
	assertFile(t, cwdPath, dir+"\n")
}

func TestRunLaunchRejectsCommandStringFlag(t *testing.T) {
	err := run([]string{
		"launch",
		"--cmd", "app --flag",
		"--session-token", "session-1",
		"--audit-correlation-id", "audit-1",
	}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("run launch error = nil, want rejection")
	}
}

func TestRunLaunchWebviewURLStartsWithoutNativeSessionFlags(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script fixture is Unix-specific")
	}
	dir := t.TempDir()
	argsPath := filepath.Join(dir, "args")
	launcher := filepath.Join(dir, "python-fixture")
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$@\" > " + shellQuote(argsPath) + "\n"
	if err := os.WriteFile(launcher, []byte(script), 0o700); err != nil {
		t.Fatalf("write launcher: %v", err)
	}
	t.Setenv("AGORA_DE_WEBVIEW_PYTHON", launcher)

	var stdout bytes.Buffer
	err := run([]string{
		"launch",
		"--url", "http://127.0.0.1:17780/shell/dist/desktop/?surface=operator",
		"--webview-title", "Agora Status",
		"--app-id", "io.agorade.ShellStatus",
		"--expected-app-id", "io.agorade.ShellStatus",
	}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("run launch webview error = %v", err)
	}
	var response launchResponse
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Status != "launched_without_surface" || response.SessionTokenPresent {
		t.Fatalf("response = %+v", response)
	}
	wantedArgs := []string{"--url", "surface=operator", "--title", "Agora Status", "--app-id", "io.agorade.ShellStatus"}
	args := waitForFileContaining(t, argsPath, wantedArgs...)
	for _, want := range wantedArgs {
		if !strings.Contains(args, want) {
			t.Fatalf("webview argv missing %q: %s", want, args)
		}
	}
}

func TestRunLaunchWaitsForPIDMatchedSurface(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script fixture is Unix-specific")
	}
	dir := t.TempDir()
	pidPath := filepath.Join(dir, "pid")
	launcher := filepath.Join(dir, "app")
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$$\" > " + shellQuote(pidPath) + "\n" +
		"sleep 1\n"
	if err := os.WriteFile(launcher, []byte(script), 0o700); err != nil {
		t.Fatalf("write launcher: %v", err)
	}

	oldListSurfaces := listSurfacesFunc
	t.Cleanup(func() { listSurfacesFunc = oldListSurfaces })
	listSurfacesFunc = func() ([]trackedSurface, error) {
		rawPID, err := os.ReadFile(pidPath)
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		pid := strings.TrimSpace(string(rawPID))
		var surface trackedSurface
		surface.Surface.ID = "view-native"
		surface.Surface.AppID = "native.app"
		surface.Surface.Title = "Native App"
		surface.Surface.Visible = true
		surface.Client.PID = atoi(pid)
		surface.Mapped = true
		surface.Visible = true
		surface.UpdatedAt = time.Now()
		return []trackedSurface{surface}, nil
	}

	var stdout bytes.Buffer
	err := run([]string{
		"launch",
		"--arg", launcher,
		"--session-token", "session-1",
		"--audit-correlation-id", "audit-1",
		"--wait-surface",
		"--wait-timeout-ms", "2000",
	}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("run launch error = %v", err)
	}
	var response launchResponse
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Status != "launched" || response.SurfaceID != "view-native" || response.Surface.Surface.ID != "view-native" {
		t.Fatalf("response = %+v", response)
	}
}

func TestRunLaunchWaitsForExpectedAppIDWhenWebKitPIDDiffers(t *testing.T) {
	oldListSurfaces := listSurfacesFunc
	t.Cleanup(func() { listSurfacesFunc = oldListSurfaces })
	listSurfacesFunc = func() ([]trackedSurface, error) {
		var surface trackedSurface
		surface.Surface.ID = "view-webkit"
		surface.Surface.AppID = "io.agorade.ShellStatus"
		surface.Surface.Title = "Agora DE Shell Status"
		surface.Surface.Visible = true
		surface.Client.PID = 999999
		surface.Mapped = true
		surface.Visible = true
		surface.UpdatedAt = time.Now()
		return []trackedSurface{surface}, nil
	}

	observation, err := waitForSurface(launchSurfaceMatch{
		RootPID:       123456,
		StartedAt:     time.Now().Add(-time.Second),
		ExpectedAppID: "io.agorade.ShellStatus",
		ExpectedTitle: "Agora DE Shell Status",
	}, 100*time.Millisecond, make(chan error))
	if err != nil {
		t.Fatalf("waitForSurface error = %v", err)
	}
	if observation.Status != "launched" || observation.Surface.Surface.ID != "view-webkit" {
		t.Fatalf("observation = %+v, want launched view-webkit", observation)
	}
}

func TestRunLaunchClassifiesReusedExistingWindow(t *testing.T) {
	oldListSurfaces := listSurfacesFunc
	t.Cleanup(func() { listSurfacesFunc = oldListSurfaces })
	startedAt := time.Now()
	listSurfacesFunc = func() ([]trackedSurface, error) {
		var surface trackedSurface
		surface.Surface.ID = "view-firefox"
		surface.Surface.AppID = "firefox"
		surface.Surface.Title = "Mozilla Firefox"
		surface.Surface.Visible = true
		surface.Client.PID = 999999
		surface.Mapped = true
		surface.Visible = true
		surface.UpdatedAt = startedAt.Add(-10 * time.Second)
		return []trackedSurface{surface}, nil
	}

	match := launchSurfaceMatch{
		RootPID:       123456,
		StartedAt:     startedAt,
		ExpectedAppID: "firefox",
		ReusableIDs:   reusableSurfaceIDs(launchSurfaceMatch{ExpectedAppID: "firefox"}),
	}
	observation, err := waitForSurface(match, 50*time.Millisecond, closedDone(nil))
	if err != nil {
		t.Fatalf("waitForSurface error = %v", err)
	}
	if observation.Status != "reused_existing_window" || observation.Surface.Surface.ID != "view-firefox" {
		t.Fatalf("observation = %+v, want reused existing Firefox", observation)
	}
}

func TestRunLaunchClassifiesSurfaceObservedAfterTimeout(t *testing.T) {
	oldListSurfaces := listSurfacesFunc
	t.Cleanup(func() { listSurfacesFunc = oldListSurfaces })
	startedAt := time.Now()
	listSurfacesFunc = func() ([]trackedSurface, error) {
		if time.Since(startedAt) < 120*time.Millisecond {
			return nil, nil
		}
		var surface trackedSurface
		surface.Surface.ID = "view-slow"
		surface.Surface.AppID = "slow.app"
		surface.Surface.Visible = true
		surface.Client.PID = 999999
		surface.Mapped = true
		surface.Visible = true
		surface.UpdatedAt = time.Now()
		return []trackedSurface{surface}, nil
	}

	observation, err := waitForSurface(launchSurfaceMatch{
		RootPID:       123456,
		StartedAt:     startedAt,
		ExpectedAppID: "slow.app",
	}, 50*time.Millisecond, make(chan error))
	if err != nil {
		t.Fatalf("waitForSurface error = %v", err)
	}
	if observation.Status != "surface_observed_after_timeout" || observation.Surface.Surface.ID != "view-slow" {
		t.Fatalf("observation = %+v, want post-timeout slow surface", observation)
	}
}

func TestRunListSurfacesCallsCompositorControlSocket(t *testing.T) {
	requests := serveControlSocket(t, func(request controlRequest) controlResponse {
		if request.Method != methodListSurfaces {
			t.Fatalf("method = %q, want %q", request.Method, methodListSurfaces)
		}
		if string(request.Body) != "null" {
			t.Fatalf("body = %s, want null", request.Body)
		}
		return controlResponse{
			OK:   true,
			Body: json.RawMessage(`{"surfaces":[{"surface":{"id":"view-1","app_id":"app","title":"App","visible":true},"client":{"pid":123},"mapped":true,"visible":true}]}`),
		}
	})

	var stdout bytes.Buffer
	err := run([]string{"list-surfaces"}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("run list-surfaces error = %v", err)
	}
	if !strings.Contains(stdout.String(), `"view-1"`) {
		t.Fatalf("stdout = %s", stdout.String())
	}
	if len(*requests) != 1 {
		t.Fatalf("requests = %d, want 1", len(*requests))
	}
}

func TestRunSurfaceFocusCallsCompositorControlSocket(t *testing.T) {
	requests := serveControlSocket(t, func(request controlRequest) controlResponse {
		return controlResponse{OK: true, Body: json.RawMessage(`{"ok":true}`)}
	})

	var stdout bytes.Buffer
	err := run([]string{"surface", "focus", "--surface", "view-focus", "--timeout-ms", "1234"}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("run surface focus error = %v", err)
	}
	if len(*requests) != 1 {
		t.Fatalf("requests = %d, want 1", len(*requests))
	}
	request := (*requests)[0]
	if request.Method != methodFocusSurface {
		t.Fatalf("method = %q, want %q", request.Method, methodFocusSurface)
	}
	var body surfaceRequest
	if err := json.Unmarshal(request.Body, &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.SurfaceID != "view-focus" || body.WaitTimeoutMs != 1234 {
		t.Fatalf("body = %+v", body)
	}
}

func TestRunSurfacePromoteCallsCompositorControlSocket(t *testing.T) {
	requests := serveControlSocket(t, func(request controlRequest) controlResponse {
		return controlResponse{OK: true, Body: json.RawMessage(`{"decision":"accepted"}`)}
	})

	var stdout bytes.Buffer
	err := run([]string{"surface", "promote", "--surface", "view-promote", "--timeout-ms", "1234"}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("run surface promote error = %v", err)
	}
	if len(*requests) != 1 {
		t.Fatalf("requests = %d, want 1", len(*requests))
	}
	request := (*requests)[0]
	if request.Method != methodPromoteSurface {
		t.Fatalf("method = %q, want %q", request.Method, methodPromoteSurface)
	}
	var body surfaceLayoutRequest
	if err := json.Unmarshal(request.Body, &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.SurfaceID != "view-promote" || body.WaitTimeoutMs != 1234 {
		t.Fatalf("body = %+v", body)
	}
}

func TestRunLayoutGetCallsCompositorControlSocket(t *testing.T) {
	requests := serveControlSocket(t, func(request controlRequest) controlResponse {
		return controlResponse{OK: true, Body: json.RawMessage(`{"layout":{"mode":"freeform","revision":1}}`)}
	})

	var stdout bytes.Buffer
	err := run([]string{"layout", "get"}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("run layout get error = %v", err)
	}
	if len(*requests) != 1 {
		t.Fatalf("requests = %d, want 1", len(*requests))
	}
	if (*requests)[0].Method != methodGetLayout {
		t.Fatalf("method = %q, want %q", (*requests)[0].Method, methodGetLayout)
	}
	if !strings.Contains(stdout.String(), `"freeform"`) {
		t.Fatalf("stdout = %s", stdout.String())
	}
}

func TestRunLayoutSetModeCallsCompositorControlSocket(t *testing.T) {
	requests := serveControlSocket(t, func(request controlRequest) controlResponse {
		return controlResponse{OK: true, Body: json.RawMessage(`{"decision":"accepted"}`)}
	})

	var stdout bytes.Buffer
	err := run([]string{"layout", "set-mode", "--mode", "zones"}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("run layout set-mode error = %v", err)
	}
	request := (*requests)[0]
	if request.Method != methodSetLayoutMode {
		t.Fatalf("method = %q, want %q", request.Method, methodSetLayoutMode)
	}
	var body setLayoutModeRequest
	if err := json.Unmarshal(request.Body, &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Mode != "zones" {
		t.Fatalf("body = %+v", body)
	}
}

func TestRunLayoutSetSettingsCallsCompositorControlSocket(t *testing.T) {
	requests := serveControlSocket(t, func(request controlRequest) controlResponse {
		return controlResponse{OK: true, Body: json.RawMessage(`{"decision":"accepted"}`)}
	})

	var stdout bytes.Buffer
	err := run([]string{
		"layout", "set-settings",
		"--rule", "dwindle",
		"--mode", "columns",
		"--outer-horizontal", "4",
		"--outer-vertical", "6",
		"--inner-horizontal", "8",
		"--inner-vertical", "10",
		"--master-count", "2",
		"--master-ratio", "0.6",
		"--smart-gaps=false",
	}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("run layout set-settings error = %v", err)
	}
	request := (*requests)[0]
	if request.Method != methodUpdateLayoutSettings {
		t.Fatalf("method = %q, want %q", request.Method, methodUpdateLayoutSettings)
	}
	var body updateLayoutSettingsRequest
	if err := json.Unmarshal(request.Body, &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Rule == nil || *body.Rule != "dwindle" || body.Mode == nil || *body.Mode != "columns" {
		t.Fatalf("body = %+v", body)
	}
	if body.Gaps == nil || body.Gaps.OuterHorizontal != 4 || body.Gaps.InnerVertical != 10 {
		t.Fatalf("gaps = %+v", body.Gaps)
	}
	if body.MasterCount == nil || *body.MasterCount != 2 || body.MasterRatio == nil || *body.MasterRatio != 0.6 || body.SmartGaps == nil || *body.SmartGaps {
		t.Fatalf("body = %+v", body)
	}
}

func TestRunSurfaceMoveResizeCallsCompositorControlSocket(t *testing.T) {
	requests := serveControlSocket(t, func(request controlRequest) controlResponse {
		return controlResponse{OK: true, Body: json.RawMessage(`{"decision":"accepted"}`)}
	})

	var stdout bytes.Buffer
	err := run([]string{
		"surface", "move-resize",
		"--surface", "view-1",
		"--x", "10",
		"--y", "20",
		"--width", "800",
		"--height", "600",
		"--timeout-ms", "1234",
	}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("run surface move-resize error = %v", err)
	}
	request := (*requests)[0]
	if request.Method != methodMoveResizeSurface {
		t.Fatalf("method = %q, want %q", request.Method, methodMoveResizeSurface)
	}
	var body surfaceLayoutRequest
	if err := json.Unmarshal(request.Body, &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.SurfaceID != "view-1" || body.Geometry == nil || body.Geometry.X != 10 || body.Geometry.Y != 20 || body.Geometry.Width != 800 || body.Geometry.Height != 600 || body.WaitTimeoutMs != 1234 {
		t.Fatalf("body = %+v", body)
	}
}

func TestRunSurfaceTileReportsBackendUnsupported(t *testing.T) {
	_ = serveControlSocket(t, func(request controlRequest) controlResponse {
		if request.Method != methodTileSurface {
			t.Fatalf("method = %q, want %q", request.Method, methodTileSurface)
		}
		return controlResponse{OK: false, ErrorClass: "backend_unsupported", ErrorMessage: "surface.tile requires compositor backend geometry authority"}
	})

	err := run([]string{"surface", "tile", "--surface", "view-1", "--zone", "primary"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "server[backend_unsupported]") {
		t.Fatalf("err = %v, want backend_unsupported", err)
	}
}

func TestRunSurfaceAssignZoneIncludesPlannerGeometry(t *testing.T) {
	requests := serveControlSocket(t, func(request controlRequest) controlResponse {
		return controlResponse{OK: true, Body: json.RawMessage(`{"decision":"accepted"}`)}
	})

	err := run([]string{"surface", "assign-zone", "--surface", "view-1", "--zone", "stack", "--x", "601", "--y", "20", "--width", "389", "--height", "378"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("run surface assign-zone: %v", err)
	}
	request := (*requests)[0]
	if request.Method != methodAssignSurfaceZone {
		t.Fatalf("method = %q, want %q", request.Method, methodAssignSurfaceZone)
	}
	var body surfaceLayoutRequest
	if err := json.Unmarshal(request.Body, &body); err != nil {
		t.Fatal(err)
	}
	if body.SurfaceID != "view-1" || body.ZoneID != "stack" || body.Geometry == nil {
		t.Fatalf("body = %+v", body)
	}
	if body.Geometry.X != 601 || body.Geometry.Y != 20 || body.Geometry.Width != 389 || body.Geometry.Height != 378 {
		t.Fatalf("geometry = %+v", body.Geometry)
	}
}

func TestRunWorkspaceActivateCallsCompositorControlSocket(t *testing.T) {
	requests := serveControlSocket(t, func(request controlRequest) controlResponse {
		return controlResponse{OK: true, Body: json.RawMessage(`{"decision":"accepted"}`)}
	})

	var stdout bytes.Buffer
	err := run([]string{"workspace", "activate", "--workspace", "workspace-1", "--output", "HDMI-A-1", "--timeout-ms", "99"}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("run workspace activate error = %v", err)
	}
	request := (*requests)[0]
	if request.Method != methodActivateWorkspace {
		t.Fatalf("method = %q, want %q", request.Method, methodActivateWorkspace)
	}
	var body workspaceRequest
	if err := json.Unmarshal(request.Body, &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.WorkspaceID != "workspace-1" || body.OutputID != "HDMI-A-1" || body.WaitTimeoutMs != 99 {
		t.Fatalf("body = %+v", body)
	}
}

func TestRunOutputCaptureCallsCompositorControlSocket(t *testing.T) {
	requests := serveControlSocket(t, func(request controlRequest) controlResponse {
		return controlResponse{OK: true, Body: json.RawMessage(`{"output":"HDMI-A-1","captures":[]}`)}
	})

	var stdout bytes.Buffer
	err := run([]string{
		"output", "capture",
		"--name", "HDMI-A-1",
		"--export",
		"--session", "session-1",
		"--session-token", "token-1",
		"--audit-correlation-id", "audit-1",
		"--evidence-class", "viewport_screenshot",
		"--asha-command-sequence-id", "seq-1",
	}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("run output capture error = %v", err)
	}
	if len(*requests) != 1 {
		t.Fatalf("requests = %d, want 1", len(*requests))
	}
	request := (*requests)[0]
	if request.Method != methodCaptureOutput {
		t.Fatalf("method = %q, want %q", request.Method, methodCaptureOutput)
	}
	var body captureOutputRequest
	if err := json.Unmarshal(request.Body, &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Name != "HDMI-A-1" || !body.Export || body.SessionID != "session-1" || body.SessionToken != "token-1" || body.AuditCorrelationID != "audit-1" || body.EvidenceClass != "viewport_screenshot" || body.ASHACommandSequenceID != "seq-1" {
		t.Fatalf("body = %+v", body)
	}
}

func assertFile(t *testing.T, path string, want string) {
	t.Helper()
	got := mustReadFile(t, path)
	if string(got) != want {
		t.Fatalf("%s = %q, want %q", path, got, want)
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return got
}

func waitForFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", path)
}

func waitForFileContaining(t *testing.T, path string, wants ...string) string {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	var last string
	for time.Now().Before(deadline) {
		got, err := os.ReadFile(path)
		if err == nil {
			last = string(got)
			missing := false
			for _, want := range wants {
				if !strings.Contains(last, want) {
					missing = true
					break
				}
			}
			if !missing {
				return last
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s to contain %q; last content: %s", path, wants, last)
	return ""
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func atoi(value string) int {
	var n int
	for _, r := range value {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
	}
	return n
}

func closedDone(err error) <-chan error {
	done := make(chan error, 1)
	done <- err
	return done
}

func serveControlSocket(t *testing.T, handle func(controlRequest) controlResponse) *[]controlRequest {
	t.Helper()
	socketPath := filepath.Join(t.TempDir(), "compositor-control.sock")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("listen unix socket: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	t.Setenv("AGORA_DE_COMPOSITOR_CONTROL_SOCKET", socketPath)

	requests := []controlRequest{}
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		var request controlRequest
		if err := json.NewDecoder(conn).Decode(&request); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		requests = append(requests, request)
		if err := json.NewEncoder(conn).Encode(handle(request)); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		<-done
	})
	return &requests
}
