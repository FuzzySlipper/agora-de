package nativelaunch

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agora-de.local/go/internal/session"
)

func TestCompositorctlBridgePassesStructuredArgv(t *testing.T) {
	dir := t.TempDir()
	callsPath := filepath.Join(dir, "calls")
	commandPath := filepath.Join(dir, "compositorctl-fixture")
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$@\" > " + shellQuote(callsPath) + "\n" +
		"printf '%s\\n' '{\"launch_id\":\"launch-1\",\"surface\":{\"surface\":{\"id\":\"view-1\"}},\"status\":\"launched\"}'\n"
	if err := os.WriteFile(commandPath, []byte(script), 0o700); err != nil {
		t.Fatalf("write fixture command: %v", err)
	}

	result, err := (CompositorctlBridge{Path: commandPath}).Launch(context.Background(), BridgeRequest{
		Args:               []string{"alacritty", "--title", "Alacritty"},
		Environment:        []string{"PATH=" + DefaultPath, "WAYLAND_DISPLAY=wayland-1"},
		WorkingDirectory:   "/home/agent",
		RequesterUID:       1000,
		RequesterGID:       1000,
		SessionToken:       session.MustToken("session-1"),
		AuditCorrelationID: "audit-1",
		OutputName:         "HDMI-A-1",
		WaitTimeout:        5 * time.Second,
	})
	if err != nil {
		t.Fatalf("Launch error = %v", err)
	}
	if result.LaunchID != "launch-1" || result.SurfaceID != "view-1" || result.Status != StatusLaunched {
		t.Fatalf("result = %+v", result)
	}

	rawCalls, err := os.ReadFile(callsPath)
	if err != nil {
		t.Fatalf("read calls: %v", err)
	}
	calls := strings.Split(strings.TrimSpace(string(rawCalls)), "\n")
	for _, forbidden := range []string{"--cmd", "alacritty --title Alacritty"} {
		for _, call := range calls {
			if call == forbidden {
				t.Fatalf("compositorctl call used forbidden shell-shaped argument %q: %#v", forbidden, calls)
			}
		}
	}
	for _, want := range []string{
		"launch",
		"--arg",
		"alacritty",
		"--arg",
		"--title",
		"--arg",
		"Alacritty",
		"--env",
		"PATH=" + DefaultPath,
		"--env",
		"WAYLAND_DISPLAY=wayland-1",
		"--cwd",
		"/home/agent",
		"--uid",
		"1000",
		"--gid",
		"1000",
		"--session-token",
		"session-1",
		"--audit-correlation-id",
		"audit-1",
		"--output",
		"HDMI-A-1",
		"--wait-surface",
		"--wait-timeout-ms",
		"5000",
	} {
		if !contains(calls, want) {
			t.Fatalf("compositorctl calls missing %q: %#v", want, calls)
		}
	}
}

func TestDecodeCompositorctlLaunchSupportsFlatSurfaceID(t *testing.T) {
	result, err := decodeCompositorctlLaunch([]byte(`{"launch_id":"launch-1","surface_id":"view-1","status":"timed_out"}`))
	if err != nil {
		t.Fatalf("decodeCompositorctlLaunch error = %v", err)
	}
	if result.LaunchID != "launch-1" || result.SurfaceID != "view-1" || result.Status != StatusTimedOut {
		t.Fatalf("result = %+v", result)
	}
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
