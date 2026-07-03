import type { FeatureManifest } from '@agora-de/domain';

export const taskbarFeatureManifest = {
  id: 'feature-taskbar',
  title: 'Taskbar',
  surfaces: ['desktop-shell'],
} as const satisfies FeatureManifest;

export const featureTaskbar = taskbarFeatureManifest.id;
