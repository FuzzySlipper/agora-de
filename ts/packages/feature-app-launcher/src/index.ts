import type { FeatureManifest } from '@agora-de/domain';

export const appLauncherFeatureManifest = {
  id: 'feature-app-launcher',
  title: 'App Launcher',
  surfaces: ['desktop-shell'],
} as const satisfies FeatureManifest;

export const featureAppLauncher = appLauncherFeatureManifest.id;
