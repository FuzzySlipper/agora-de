export interface SurfaceSummary {
  readonly id: string;
  readonly ownerUid: number;
}

export type ShellSurface = 'desktop-shell' | 'operator-console';

export interface FeatureManifest {
  readonly id: string;
  readonly title: string;
  readonly surfaces: readonly ShellSurface[];
}
