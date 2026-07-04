import type { FeatureManifest } from '@agora-de/domain';
import type { CatalogApp } from '@agora-de/protocol';
import type { AsyncState } from '@agora-de/store';

export const appLauncherFeatureManifest = {
  id: 'feature-app-launcher',
  title: 'App Launcher',
  surfaces: ['desktop-shell'],
} as const satisfies FeatureManifest;

export const featureAppLauncher = appLauncherFeatureManifest.id;

export interface AppLauncherItemView {
  readonly id: string;
  readonly label: string;
  readonly icon: string;
}

export interface AppLauncherViewModel {
  readonly apps: readonly AppLauncherItemView[];
  readonly empty: boolean;
}

export function appLauncherViewModel(
  state: AsyncState<readonly CatalogApp[]>,
): AsyncState<AppLauncherViewModel> {
  if (state.kind === 'idle') {
    return { kind: 'idle' };
  }
  if (state.kind === 'loading') {
    return { kind: 'loading' };
  }
  if (state.kind === 'error') {
    return { kind: 'error', error: state.error };
  }

  const apps = state.value.map((app) => ({
    id: app.id,
    label: app.name,
    icon: app.icon,
  }));

  return {
    kind: 'data',
    value: {
      apps,
      empty: apps.length === 0,
    },
  };
}
