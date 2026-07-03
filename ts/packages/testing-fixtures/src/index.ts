import type { CaptureClassification, EvidencePacket } from '@agora-de/protocol';

export const fixtureWorkspace = 'agora-de';

export const visibleCaptureClassification: CaptureClassification = 'capture_visible';

export const visibleEvidencePacket: EvidencePacket = {
  scenario: 'capture-visible-fixture',
  capturedAtUnixMillis: 0,
  visualStatus: 'visible',
  captureClassification: visibleCaptureClassification,
};
