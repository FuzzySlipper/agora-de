package agenthealth

import (
	"context"

	"agora-de.local/go/internal/osboundary"
)

type AgentView struct {
	UID         uint32
	DisplayName string
	State       osboundary.AgentState
}

type Summary struct {
	Agents  []AgentView
	Ready   int
	Busy    int
	Offline int
	Unknown int
}

func BuildSummary(ctx context.Context, client osboundary.AgentClient) (Summary, error) {
	agents, err := client.ListAgents(ctx)
	if err != nil {
		return Summary{}, err
	}

	summary := Summary{Agents: make([]AgentView, 0, len(agents))}
	for _, agent := range agents {
		summary.Agents = append(summary.Agents, AgentView{
			UID:         agent.UID,
			DisplayName: agent.DisplayName,
			State:       agent.State,
		})
		switch agent.State {
		case osboundary.AgentStateReady:
			summary.Ready++
		case osboundary.AgentStateBusy:
			summary.Busy++
		case osboundary.AgentStateOffline:
			summary.Offline++
		default:
			summary.Unknown++
		}
	}
	return summary, nil
}

