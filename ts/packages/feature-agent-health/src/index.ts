import type { FeatureManifest } from '@agora-de/domain';
import type { AgentInfo, AgentState } from '@agora-de/protocol';
import type { AsyncState } from '@agora-de/store';

export const agentHealthFeatureManifest = {
  id: 'feature-agent-health',
  title: 'Agent Health',
  surfaces: ['operator-console'],
} as const satisfies FeatureManifest;

export const featureAgentHealth = agentHealthFeatureManifest.id;

export interface AgentHealthViewModel {
  readonly agents: readonly AgentInfo[];
  readonly counts: Readonly<Record<AgentState, number>>;
  readonly ready: boolean;
}

export function agentHealthViewModel(
  state: AsyncState<readonly AgentInfo[]>,
): AsyncState<AgentHealthViewModel> {
  if (state.kind === 'idle') {
    return { kind: 'idle' };
  }
  if (state.kind === 'loading') {
    return { kind: 'loading' };
  }
  if (state.kind === 'error') {
    return { kind: 'error', error: state.error };
  }

  const counts: Record<AgentState, number> = {
    unknown: 0,
    ready: 0,
    busy: 0,
    offline: 0,
  };
  for (const agent of state.value) {
    counts[agent.state] = (counts[agent.state] ?? 0) + 1;
  }

  return {
    kind: 'data',
    value: {
      agents: state.value,
      counts,
      ready: counts.offline === 0 && counts.unknown === 0,
    },
  };
}
