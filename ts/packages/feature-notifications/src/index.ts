import type { FeatureManifest } from '@agora-de/domain';

export const notificationsFeatureManifest = {
  id: 'feature-notifications',
  title: 'Notifications',
  surfaces: ['desktop-shell'],
} as const satisfies FeatureManifest;

export const featureNotifications = notificationsFeatureManifest.id;
