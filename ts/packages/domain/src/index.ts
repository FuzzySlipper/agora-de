import type { SurfaceEvent, SurfaceEventKind } from '@agora-de/protocol';

export interface SurfaceSummary {
  readonly id: string;
  readonly ownerUid: number;
}

export interface SurfaceLifecycleView extends SurfaceSummary {
  readonly mapped: boolean;
  readonly focused: boolean;
  readonly inputDeniedCount: number;
  readonly lastEventKind: SurfaceEventKind;
}

export type ShellSurface = 'desktop-shell' | 'operator-console';

export interface FeatureManifest {
  readonly id: string;
  readonly title: string;
  readonly surfaces: readonly ShellSurface[];
}

interface MutableSurfaceLifecycleView {
  id: string;
  ownerUid: number;
  mapped: boolean;
  focused: boolean;
  inputDeniedCount: number;
  lastEventKind: SurfaceEventKind;
}

export function projectSurfaceLifecycle(events: readonly SurfaceEvent[]): readonly SurfaceLifecycleView[] {
  const surfaces = new Map<string, MutableSurfaceLifecycleView>();
  const order: string[] = [];

  for (const event of events) {
    let surface = surfaces.get(event.surfaceId);
    if (!surface) {
      surface = {
        id: event.surfaceId,
        ownerUid: event.ownerUid,
        mapped: false,
        focused: false,
        inputDeniedCount: 0,
        lastEventKind: event.kind,
      };
      surfaces.set(event.surfaceId, surface);
      order.push(event.surfaceId);
    }

    surface.ownerUid = event.ownerUid;
    surface.lastEventKind = event.kind;

    if (event.kind === 'mapped') {
      surface.mapped = true;
    } else if (event.kind === 'unmapped') {
      surface.mapped = false;
      surface.focused = false;
    } else if (event.kind === 'focused') {
      for (const candidate of surfaces.values()) {
        candidate.focused = false;
      }
      surface.mapped = true;
      surface.focused = true;
    } else if (event.kind === 'input_denied') {
      surface.inputDeniedCount += 1;
    }
  }

  return order.map((id) => {
    const surface = surfaces.get(id);
    if (!surface) {
      throw new Error(`surface order referenced missing surface ${id}`);
    }
    return { ...surface };
  });
}
