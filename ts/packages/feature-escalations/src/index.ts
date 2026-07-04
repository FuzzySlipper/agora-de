import type { FeatureManifest } from '@agora-de/domain';
import type { AdminEscalationSummary } from '@agora-de/protocol';
import type { AsyncState } from '@agora-de/store';

export const escalationsFeatureManifest = {
  id: 'feature-escalations',
  title: 'Escalations',
  surfaces: ['operator-console'],
} as const satisfies FeatureManifest;

export const featureEscalations = escalationsFeatureManifest.id;

export interface EscalationItemView {
  readonly id: string;
  readonly requesterUid: number;
  readonly summary: string;
  readonly ageMillis: number;
}

export interface EscalationsViewModel {
  readonly pending: readonly EscalationItemView[];
  readonly hasPending: boolean;
}

export function escalationsViewModel(
  state: AsyncState<readonly AdminEscalationSummary[]>,
  nowUnixMillis: number,
): AsyncState<EscalationsViewModel> {
  if (state.kind === 'idle') {
    return { kind: 'idle' };
  }
  if (state.kind === 'loading') {
    return { kind: 'loading' };
  }
  if (state.kind === 'error') {
    return { kind: 'error', error: state.error };
  }

  const pending = state.value.map((item) => ({
    id: item.id,
    requesterUid: item.actorUid,
    summary: item.summary,
    ageMillis: Math.max(0, nowUnixMillis - item.requestedAtUnixMillis),
  }));

  return {
    kind: 'data',
    value: {
      pending,
      hasPending: pending.length > 0,
    },
  };
}
