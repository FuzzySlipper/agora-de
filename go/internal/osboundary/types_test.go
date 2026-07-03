package osboundary

import (
	"context"
	"errors"
	"testing"
)

func TestValidateDecisionRequest(t *testing.T) {
	valid := EscalationDecisionRequest{
		EscalationID: "esc-1",
		Decision:     EscalationDecisionApprove,
		DeciderUID:   1000,
		Reason:       "approved by operator",
	}
	if err := ValidateDecisionRequest(valid); err != nil {
		t.Fatalf("valid request rejected: %v", err)
	}

	invalid := valid
	invalid.Decision = "maybe"
	if !errors.Is(ValidateDecisionRequest(invalid), ErrInvalidDecision) {
		t.Fatalf("invalid decision was not classified as ErrInvalidDecision")
	}
}

func TestUnavailableClientFailsClosed(t *testing.T) {
	client := UnavailableClient{}

	if _, err := client.ListPendingEscalations(context.Background()); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("ListPendingEscalations error = %v, want ErrUnavailable", err)
	}

	_, errs := client.StreamAuditEvents(context.Background())
	err, ok := <-errs
	if !ok || !errors.Is(err, ErrUnavailable) {
		t.Fatalf("StreamAuditEvents error = %v, ok = %v, want ErrUnavailable", err, ok)
	}
}

