import type { FeatureManifest } from '@agora-de/domain';
import type { SurfaceLifecycleView } from '@agora-de/domain';
import type { AsyncState } from '@agora-de/store';

export const workSurfaceControlsFeatureManifest = {
  id: 'feature-work-surface-controls',
  title: 'Work Surface Controls',
  surfaces: ['desktop-shell'],
} as const satisfies FeatureManifest;

export const featureWorkSurfaceControls = workSurfaceControlsFeatureManifest.id;

export interface WorkSurfaceControlsViewModel {
  readonly surfaceCount: number;
  readonly focusedSurfaceId?: string;
  readonly deniedInputSurfaceIds: readonly string[];
}

export function workSurfaceControlsViewModel(
  state: AsyncState<readonly SurfaceLifecycleView[]>,
): AsyncState<WorkSurfaceControlsViewModel> {
  if (state.kind === 'idle') {
    return { kind: 'idle' };
  }
  if (state.kind === 'loading') {
    return { kind: 'loading' };
  }
  if (state.kind === 'error') {
    return { kind: 'error', error: state.error };
  }

  const visibleSurfaces = state.value.filter((surface) => surface.mapped);
  const focusedSurface = visibleSurfaces.find((surface) => surface.focused);
  const deniedInputSurfaceIds = visibleSurfaces
    .filter((surface) => surface.inputDeniedCount > 0)
    .map((surface) => surface.id);
  const value: WorkSurfaceControlsViewModel = {
    surfaceCount: visibleSurfaces.length,
    deniedInputSurfaceIds,
  };

  return {
    kind: 'data',
    value: focusedSurface ? { ...value, focusedSurfaceId: focusedSurface.id } : value,
  };
}
