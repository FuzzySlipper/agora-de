import { projectSurfaceLifecycle } from '@agora-de/domain';
import { dataState, surfaceLifecycleState } from '@agora-de/store';
import { workSurfaceControlsViewModel } from '@agora-de/feature-work-surface-controls';
import type { CaptureClassification, EvidencePacket, SurfaceEvent } from '@agora-de/protocol';

export const fixtureWorkspace = 'agora-de';

export const visibleCaptureClassification: CaptureClassification = 'capture_visible';

export const visibleEvidencePacket: EvidencePacket = {
  scenario: 'capture-visible-fixture',
  capturedAtUnixMillis: 0,
  visualStatus: 'visible',
  captureClassification: visibleCaptureClassification,
};

export const surfaceLifecycleEvents: readonly SurfaceEvent[] = [
  { surfaceId: 'view-42', kind: 'mapped', ownerUid: 60001 },
  { surfaceId: 'view-42', kind: 'focused', ownerUid: 60001 },
  { surfaceId: 'view-42', kind: 'input_denied', ownerUid: 60001 },
];

export const surfaceLifecycleViewFixture = projectSurfaceLifecycle(surfaceLifecycleEvents);

export const surfaceLifecycleStateFixture = surfaceLifecycleState({
  ok: true,
  value: surfaceLifecycleEvents,
});

export const workSurfaceControlsViewModelFixture = workSurfaceControlsViewModel(
  dataState(surfaceLifecycleViewFixture),
);

export function assertSurfaceLifecycleFixture(): void {
  const [surface] = surfaceLifecycleViewFixture;
  if (!surface || surface.id !== 'view-42') {
    throw new Error('surface lifecycle fixture should track view-42');
  }
  if (!surface.mapped || !surface.focused) {
    throw new Error('surface lifecycle fixture should keep view-42 mapped and focused');
  }
  if (surface.ownerUid !== 60001 || surface.inputDeniedCount !== 1) {
    throw new Error('surface lifecycle fixture should preserve owner uid and denied input count');
  }

  if (workSurfaceControlsViewModelFixture.kind !== 'data') {
    throw new Error('work surface controls fixture should produce data state');
  }
  if (
    workSurfaceControlsViewModelFixture.value.surfaceCount !== 1 ||
    workSurfaceControlsViewModelFixture.value.focusedSurfaceId !== 'view-42' ||
    workSurfaceControlsViewModelFixture.value.deniedInputSurfaceIds[0] !== 'view-42'
  ) {
    throw new Error('work surface controls fixture should summarize view-42 lifecycle state');
  }
}
