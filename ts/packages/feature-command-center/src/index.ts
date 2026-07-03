import type { FeatureManifest } from '@agora-de/domain';

export const commandCenterFeatureManifest = {
  id: 'feature-command-center',
  title: 'Command Center',
  surfaces: ['desktop-shell', 'operator-console'],
} as const satisfies FeatureManifest;

export const featureCommandCenter = commandCenterFeatureManifest.id;
