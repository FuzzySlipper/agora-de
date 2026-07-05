import type { FeatureManifest } from '@agora-de/domain';
import { themeVar } from '@agora-de/theme';

export const taskbarFeatureManifest = {
  id: 'feature-taskbar',
  title: 'Taskbar',
  surfaces: ['desktop-shell'],
} as const satisfies FeatureManifest;

export const featureTaskbar = taskbarFeatureManifest.id;

export const taskbarThemeVars = {
  background: themeVar('surface'),
  border: themeVar('evidenceAccent'),
  foreground: themeVar('foreground'),
  gap: themeVar('panelGap'),
  height: themeVar('panelHeight'),
  itemBackground: themeVar('surfaceRaised'),
  itemBorder: themeVar('borderSubtle'),
} as const;
