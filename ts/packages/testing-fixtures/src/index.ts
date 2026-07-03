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
  { surfaceId: 'surface-1', kind: 'mapped', ownerUid: 1000 },
  { surfaceId: 'surface-2', kind: 'mapped', ownerUid: 1001 },
  { surfaceId: 'surface-2', kind: 'focused', ownerUid: 1001 },
  { surfaceId: 'surface-1', kind: 'input_denied', ownerUid: 1000 },
  { surfaceId: 'surface-1', kind: 'unmapped', ownerUid: 1000 },
];

export const surfaceLifecycleViewFixture = projectSurfaceLifecycle(surfaceLifecycleEvents);

export const surfaceLifecycleStateFixture = surfaceLifecycleState({
  ok: true,
  value: surfaceLifecycleEvents,
});

export const workSurfaceControlsViewModelFixture = workSurfaceControlsViewModel(
  dataState(surfaceLifecycleViewFixture),
);
