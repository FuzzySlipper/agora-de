package osboundary

import "context"

type UnavailableClient struct{}

func (UnavailableClient) ListPendingEscalations(context.Context) ([]AdminEscalationEvent, error) {
	return nil, ErrUnavailable
}

func (UnavailableClient) SubmitDecision(context.Context, EscalationDecisionRequest) (EscalationDecisionResponse, error) {
	return EscalationDecisionResponse{}, ErrUnavailable
}

func (UnavailableClient) StreamAuditEvents(context.Context) (<-chan AuditEvent, <-chan error) {
	events := make(chan AuditEvent)
	errs := make(chan error, 1)
	errs <- ErrUnavailable
	close(events)
	close(errs)
	return events, errs
}

func (UnavailableClient) ListAgents(context.Context) ([]AgentInfo, error) {
	return nil, ErrUnavailable
}

