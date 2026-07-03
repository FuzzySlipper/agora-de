import type { FeatureManifest } from '@agora-de/domain';

export const workSurfaceControlsFeatureManifest = {
  id: 'feature-work-surface-controls',
  title: 'Work Surface Controls',
  surfaces: ['desktop-shell'],
} as const satisfies FeatureManifest;

export const featureWorkSurfaceControls = workSurfaceControlsFeatureManifest.id;
