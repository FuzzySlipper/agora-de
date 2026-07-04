import type { FeatureManifest } from '@agora-de/domain';
import type { AuditEvent } from '@agora-de/protocol';
import type { AsyncState } from '@agora-de/store';

export const auditTailFeatureManifest = {
  id: 'feature-audit-tail',
  title: 'Audit Tail',
  surfaces: ['operator-console'],
} as const satisfies FeatureManifest;

export const featureAuditTail = auditTailFeatureManifest.id;

export interface AuditTailItemView {
  readonly id: string;
  readonly actorUid: number;
  readonly label: string;
  readonly createdAtUnixMillis: number;
}

export interface AuditTailViewModel {
  readonly events: readonly AuditTailItemView[];
  readonly empty: boolean;
}

export function auditTailViewModel(
  state: AsyncState<readonly AuditEvent[]>,
): AsyncState<AuditTailViewModel> {
  if (state.kind === 'idle') {
    return { kind: 'idle' };
  }
  if (state.kind === 'loading') {
    return { kind: 'loading' };
  }
  if (state.kind === 'error') {
    return { kind: 'error', error: state.error };
  }

  const events = state.value.map((event) => ({
    id: event.id,
    actorUid: event.actorUid,
    label: `${event.action} ${event.subject}`,
    createdAtUnixMillis: event.createdAtUnixMillis,
  }));

  return {
    kind: 'data',
    value: {
      events,
      empty: events.length === 0,
    },
  };
}
