import { agentHealthFeatureManifest } from '@agora-de/feature-agent-health';
import { appLauncherFeatureManifest } from '@agora-de/feature-app-launcher';
import { auditTailFeatureManifest } from '@agora-de/feature-audit-tail';
import { commandCenterFeatureManifest } from '@agora-de/feature-command-center';
import { escalationsFeatureManifest } from '@agora-de/feature-escalations';
import { notificationsFeatureManifest } from '@agora-de/feature-notifications';
import { taskbarFeatureManifest } from '@agora-de/feature-taskbar';
import { workSurfaceControlsFeatureManifest } from '@agora-de/feature-work-surface-controls';
import type { FeatureManifest, ShellSurface } from '@agora-de/domain';

export const shellName = 'agora-de-shell';

export interface ShellComposition {
  readonly name: string;
  readonly surface: ShellSurface;
  readonly features: readonly FeatureManifest[];
}

const featureManifests: readonly FeatureManifest[] = [
  taskbarFeatureManifest,
  appLauncherFeatureManifest,
  workSurfaceControlsFeatureManifest,
  notificationsFeatureManifest,
  commandCenterFeatureManifest,
  agentHealthFeatureManifest,
  escalationsFeatureManifest,
  auditTailFeatureManifest,
];

export function composeShell(surface: ShellSurface): ShellComposition {
  return {
    name: shellName,
    surface,
    features: featureManifests.filter((feature) => feature.surfaces.includes(surface)),
  };
}

export const desktopShellComposition = composeShell('desktop-shell');
export const operatorConsoleComposition = composeShell('operator-console');
