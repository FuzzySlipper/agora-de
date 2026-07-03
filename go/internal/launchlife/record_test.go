package launchlife

import (
	"errors"
	"testing"

	"agora-de.local/go/internal/session"
)

func TestNewRecordStartsPendingWithSessionToken(t *testing.T) {
	token := session.MustToken("session-1")
	record, err := NewRecord("launch-1", 60001, token)
	if err != nil {
		t.Fatal(err)
	}
	if record.State != StatePending {
		t.Fatalf("state = %q, want %q", record.State, StatePending)
	}
	if record.SessionToken != token {
		t.Fatalf("session token = %q, want %q", record.SessionToken, token)
	}
}

func TestNewRecordRejectsMissingToken(t *testing.T) {
	_, err := NewRecord("launch-1", 60001, "")
	if !errors.Is(err, ErrInvalidLaunch) {
		t.Fatalf("missing token error = %v, want ErrInvalidLaunch", err)
	}
}

func TestWithMappedSurface(t *testing.T) {
	record, err := NewRecord("launch-1", 60001, session.MustToken("session-1"))
	if err != nil {
		t.Fatal(err)
	}

	mapped, err := record.WithMappedSurface("view-42")
	if err != nil {
		t.Fatal(err)
	}
	if mapped.State != StateSurfaceMapped {
		t.Fatalf("state = %q, want %q", mapped.State, StateSurfaceMapped)
	}
	if mapped.SurfaceID != "view-42" {
		t.Fatalf("surface id = %q, want view-42", mapped.SurfaceID)
	}
}

