package agenthealth

import (
	"context"
	"errors"
	"testing"

	"agora-de.local/go/internal/osboundary"
)

func TestBuildSummaryUsesTypedAgentClient(t *testing.T) {
	client := fakeAgentClient{
		agents: []osboundary.AgentInfo{
			{UID: 60001, DisplayName: "Ready Agent", State: osboundary.AgentStateReady},
			{UID: 60002, DisplayName: "Busy Agent", State: osboundary.AgentStateBusy},
			{UID: 60003, DisplayName: "Offline Agent", State: osboundary.AgentStateOffline},
			{UID: 60004, DisplayName: "Unknown Agent", State: osboundary.AgentStateUnknown},
		},
	}

	summary, err := BuildSummary(context.Background(), client)
	if err != nil {
		t.Fatal(err)
	}
	if len(summary.Agents) != 4 {
		t.Fatalf("agents = %d, want 4", len(summary.Agents))
	}
	if summary.Ready != 1 || summary.Busy != 1 || summary.Offline != 1 || summary.Unknown != 1 {
		t.Fatalf("counts = ready:%d busy:%d offline:%d unknown:%d", summary.Ready, summary.Busy, summary.Offline, summary.Unknown)
	}
}

func TestBuildSummaryPropagatesBoundaryError(t *testing.T) {
	_, err := BuildSummary(context.Background(), fakeAgentClient{err: osboundary.ErrUnavailable})
	if !errors.Is(err, osboundary.ErrUnavailable) {
		t.Fatalf("error = %v, want ErrUnavailable", err)
	}
}

type fakeAgentClient struct {
	agents []osboundary.AgentInfo
	err    error
}

func (client fakeAgentClient) ListAgents(context.Context) ([]osboundary.AgentInfo, error) {
	return client.agents, client.err
}

