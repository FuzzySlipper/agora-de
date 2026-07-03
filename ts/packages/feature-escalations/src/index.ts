import type { FeatureManifest } from '@agora-de/domain';

export const escalationsFeatureManifest = {
  id: 'feature-escalations',
  title: 'Escalations',
  surfaces: ['operator-console'],
} as const satisfies FeatureManifest;

export const featureEscalations = escalationsFeatureManifest.id;
