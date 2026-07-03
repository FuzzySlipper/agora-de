package osboundary

import (
	"context"
	"errors"
	"time"
)

var (
	ErrUnavailable       = errors.New("agora-os boundary unavailable")
	ErrInvalidDecision   = errors.New("invalid escalation decision")
	ErrUnauthenticated   = errors.New("agora-os boundary unauthenticated")
	ErrUnsupportedLegacy = errors.New("legacy log-file boundary is unsupported")
)

type AgentInfo struct {
	UID         uint32
	DisplayName string
	State       AgentState
}

type AgentState string

const (
	AgentStateUnknown AgentState = "unknown"
	AgentStateReady   AgentState = "ready"
	AgentStateBusy    AgentState = "busy"
	AgentStateOffline AgentState = "offline"
)

type AuditEvent struct {
	ID        string
	ActorUID  uint32
	Action    string
	Subject   string
	CreatedAt time.Time
}

type AdminEscalationEvent struct {
	ID          string
	RequesterUID uint32
	Summary     string
	CreatedAt   time.Time
}

type EscalationDecision string

const (
	EscalationDecisionApprove EscalationDecision = "approve"
	EscalationDecisionDeny    EscalationDecision = "deny"
)

type EscalationDecisionRequest struct {
	EscalationID string
	Decision     EscalationDecision
	DeciderUID   uint32
	Reason       string
}

type EscalationDecisionResponse struct {
	EscalationID string
	Accepted     bool
}

type EscalationClient interface {
	ListPendingEscalations(ctx context.Context) ([]AdminEscalationEvent, error)
	SubmitDecision(ctx context.Context, request EscalationDecisionRequest) (EscalationDecisionResponse, error)
}

type AuditClient interface {
	StreamAuditEvents(ctx context.Context) (<-chan AuditEvent, <-chan error)
}

type AgentClient interface {
	ListAgents(ctx context.Context) ([]AgentInfo, error)
}

type Client interface {
	EscalationClient
	AuditClient
	AgentClient
}

