package main

import (
	"bytes"
	"encoding/json"
	"errors"
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

func assertFile(t *testing.T, path string, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(got) != want {
		t.Fatalf("%s = %q, want %q", path, got, want)
	}
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
