package nativelaunch

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestBuildEnvironmentKeepsOnlyAllowlistedVariables(t *testing.T) {
	got := BuildEnvironment(map[string]string{
		"AGORA_SECRET":             "nope",
		"HOME":                     "/home/agent",
		"LANG":                     "en_US.UTF-8",
		"LC_TIME":                  "C",
		"PATH":                     "/tmp/evil",
		"WAYLAND_DISPLAY":          "wayland-1",
		"XDG_RUNTIME_DIR":          "/run/user/1000",
		"XDG_CURRENT_DESKTOP":      "agora-de",
		"XDG_SESSION_TYPE":         "wayland",
		"DBUS_SESSION_BUS_ADDRESS": "unix:path=/run/user/1000/bus",
	})
	want := []string{
		"DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/1000/bus",
		"HOME=/home/agent",
		"LANG=en_US.UTF-8",
		"LC_TIME=C",
		"PATH=" + DefaultPath,
		"WAYLAND_DISPLAY=wayland-1",
		"XDG_CURRENT_DESKTOP=agora-de",
		"XDG_RUNTIME_DIR=/run/user/1000",
		"XDG_SESSION_TYPE=wayland",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("environment = %#v, want %#v", got, want)
	}
}

func TestResolveWorkingDirectoryDefaultsToHome(t *testing.T) {
	home := t.TempDir()
	got, err := ResolveWorkingDirectory("", home)
	if err != nil {
		t.Fatalf("ResolveWorkingDirectory error = %v", err)
	}
	if got != home {
		t.Fatalf("working directory = %q, want %q", got, home)
	}
}

func TestResolveWorkingDirectoryRejectsRelativeOrMissingPath(t *testing.T) {
	if _, err := ResolveWorkingDirectory("relative", ""); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("relative path error = %v, want ErrInvalidRequest", err)
	}
	missing := filepath.Join(t.TempDir(), "missing")
	if _, err := ResolveWorkingDirectory(missing, ""); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("missing path error = %v, want ErrInvalidRequest", err)
	}
}

func TestResolveWorkingDirectoryRejectsFiles(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if _, err := ResolveWorkingDirectory(file, ""); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("file path error = %v, want ErrInvalidRequest", err)
	}
}
