import type { FeatureManifest } from '@agora-de/domain';

export const auditTailFeatureManifest = {
  id: 'feature-audit-tail',
  title: 'Audit Tail',
  surfaces: ['operator-console'],
} as const satisfies FeatureManifest;

export const featureAuditTail = auditTailFeatureManifest.id;
