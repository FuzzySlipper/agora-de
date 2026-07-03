package session

import (
	"errors"
	"testing"
)

func TestNewTokenRejectsBlank(t *testing.T) {
	if _, err := NewToken("  "); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("NewToken blank error = %v, want ErrInvalidToken", err)
	}
}

func TestStoreCreateRequireDestroy(t *testing.T) {
	store := NewStore()
	token := MustToken("session-1")

	record := store.Create(token, 60001)
	if record.RequesterUID != 60001 {
		t.Fatalf("requester uid = %d, want 60001", record.RequesterUID)
	}

	required, err := store.Require(token)
	if err != nil {
		t.Fatal(err)
	}
	if required.Token != token {
		t.Fatalf("required token = %q, want %q", required.Token, token)
	}

	if !store.Destroy(token) {
		t.Fatal("Destroy returned false for existing token")
	}
	if _, err := store.Require(token); !errors.Is(err, ErrUnknownSession) {
		t.Fatalf("Require destroyed token error = %v, want ErrUnknownSession", err)
	}
}

