package nativelaunch

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"agora-de.local/go/internal/appcatalog"
	"agora-de.local/go/internal/session"
)

func TestLauncherCallsBridgeWithStructuredRequest(t *testing.T) {
	bridge := &recordingBridge{
		result: BridgeResult{LaunchID: "launch-1", SurfaceID: "view-1", Status: StatusLaunched},
	}
	launcher := New(bridge)
	home := t.TempDir()

	result, err := launcher.Launch(context.Background(), Request{
		Entry: appcatalog.Entry{
			Type:          "Application",
			Name:          "Alacritty",
			Exec:          `alacritty --title "%c"`,
			ExecTokens:    []string{"alacritty", "--title"},
			ExecSupported: true,
		},
		RequesterUID:       1000,
		RequesterGID:       1000,
		SessionToken:       session.MustToken("session-1"),
		AuditCorrelationID: "audit-1",
		OutputName:         "HDMI-A-1",
		HomeDirectory:      home,
		BaseEnvironment: map[string]string{
			"HOME":            home,
			"WAYLAND_DISPLAY": "wayland-1",
			"AGORA_SECRET":    "nope",
		},
	})
	if err != nil {
		t.Fatalf("Launch error = %v", err)
	}
	if result.LaunchID != "launch-1" || result.SurfaceID != "view-1" || result.Status != StatusLaunched {
		t.Fatalf("result = %+v", result)
	}
	if !reflect.DeepEqual(bridge.request.Args, []string{"alacritty", "--title", "Alacritty"}) {
		t.Fatalf("bridge args = %#v", bridge.request.Args)
	}
	if bridge.request.WorkingDirectory != home {
		t.Fatalf("bridge cwd = %q, want %q", bridge.request.WorkingDirectory, home)
	}
	if bridge.request.SessionToken != session.MustToken("session-1") {
		t.Fatalf("bridge session token = %q", bridge.request.SessionToken)
	}
	if bridge.request.OutputName != "HDMI-A-1" || bridge.request.AuditCorrelationID != "audit-1" {
		t.Fatalf("bridge metadata = %+v", bridge.request)
	}
	if bridge.request.WaitTimeout != DefaultWaitTimeout {
		t.Fatalf("bridge wait timeout = %s, want %s", bridge.request.WaitTimeout, DefaultWaitTimeout)
	}
	if reflect.DeepEqual(bridge.request.Environment, []string{"AGORA_SECRET=nope"}) {
		t.Fatalf("bridge environment leaked secret: %#v", bridge.request.Environment)
	}
}

func TestLauncherRejectsInvalidRequestBeforeBridge(t *testing.T) {
	bridge := &recordingBridge{}
	launcher := New(bridge)

	_, err := launcher.Launch(context.Background(), Request{
		Entry: appcatalog.Entry{
			Type:          "Application",
			Name:          "Alacritty",
			Exec:          "alacritty",
			ExecTokens:    []string{"alacritty"},
			ExecSupported: true,
		},
		RequesterUID:       1000,
		RequesterGID:       1000,
		AuditCorrelationID: "audit-1",
		HomeDirectory:      t.TempDir(),
	})
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Launch error = %v, want ErrInvalidRequest", err)
	}
	if bridge.called {
		t.Fatal("bridge should not be called for invalid request")
	}
}

func TestLauncherReturnsFailedWhenBridgeFails(t *testing.T) {
	bridge := &recordingBridge{err: errors.New("bridge down")}
	launcher := New(bridge)

	result, err := launcher.Launch(context.Background(), validRequest(t))
	if err == nil {
		t.Fatal("Launch error = nil, want bridge error")
	}
	if result.Status != StatusFailed {
		t.Fatalf("status = %q, want %q", result.Status, StatusFailed)
	}
}

func TestLauncherRejectsLaunchedResultWithoutSurface(t *testing.T) {
	bridge := &recordingBridge{
		result: BridgeResult{LaunchID: "launch-1", Status: StatusLaunched},
	}
	launcher := New(bridge)

	_, err := launcher.Launch(context.Background(), validRequest(t))
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Launch error = %v, want ErrInvalidRequest", err)
	}
}

func TestLauncherPreservesExplicitWaitTimeout(t *testing.T) {
	bridge := &recordingBridge{
		result: BridgeResult{LaunchID: "launch-1", SurfaceID: "view-1", Status: StatusLaunched},
	}
	launcher := New(bridge)
	request := validRequest(t)
	request.WaitTimeout = 2 * time.Second

	if _, err := launcher.Launch(context.Background(), request); err != nil {
		t.Fatalf("Launch error = %v", err)
	}
	if bridge.request.WaitTimeout != 2*time.Second {
		t.Fatalf("wait timeout = %s, want 2s", bridge.request.WaitTimeout)
	}
}

func validRequest(t *testing.T) Request {
	t.Helper()
	return Request{
		Entry: appcatalog.Entry{
			Type:          "Application",
			Name:          "Alacritty",
			Exec:          "alacritty",
			ExecTokens:    []string{"alacritty"},
			ExecSupported: true,
		},
		RequesterUID:       1000,
		RequesterGID:       1000,
		SessionToken:       session.MustToken("session-1"),
		AuditCorrelationID: "audit-1",
		HomeDirectory:      t.TempDir(),
	}
}

type recordingBridge struct {
	called  bool
	request BridgeRequest
	result  BridgeResult
	err     error
}

func (bridge *recordingBridge) Launch(_ context.Context, request BridgeRequest) (BridgeResult, error) {
	bridge.called = true
	bridge.request = request
	return bridge.result, bridge.err
}
