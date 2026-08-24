import { appLauncherViewModel } from '@agora-de/feature-app-launcher';
import { appLauncherThemeVars } from '@agora-de/feature-app-launcher';
import { agentHealthViewModel } from '@agora-de/feature-agent-health';
import { auditTailViewModel } from '@agora-de/feature-audit-tail';
import { escalationsViewModel } from '@agora-de/feature-escalations';
import { taskbarThemeVars } from '@agora-de/feature-taskbar';
import { projectSurfaceLifecycle } from '@agora-de/domain';
import { dataState, appCatalogState, surfaceLifecycleState } from '@agora-de/store';
import { desktopShellComposition, operatorConsoleComposition } from '@agora-de/shell';
import { evidenceThemeTokenNames, liveThemeClientScript, shellThemeTokens } from '@agora-de/theme';
import { catalogAppsPath, decodeCatalogAppsResponse } from '@agora-de/transport';
import { workSurfaceControlsViewModel } from '@agora-de/feature-work-surface-controls';
import type {
  AdminEscalationSummary,
  AgentInfo,
  AuditEvent,
  CatalogAppsResponse,
  CaptureClassification,
  EvidencePacket,
  DiagnosticsApplyRequest,
  DiagnosticsSettingsState,
  SettingsCatalogResponse,
  SurfaceEvent,
} from '@agora-de/protocol';

export const fixtureWorkspace = 'agora-de';

export const visibleCaptureClassification: CaptureClassification = 'capture_visible';

export const visibleEvidencePacket: EvidencePacket = {
  scenario: 'capture-visible-fixture',
  capturedAtUnixMillis: 0,
  visualStatus: 'visible',
  captureClassification: visibleCaptureClassification,
};

export const settingsCatalogFixture: SettingsCatalogResponse = {
  schemaVersion: 1,
  modules: [
    {
      manifest: {
        id: 'diagnostics',
        category: 'system',
        title: 'Diagnostics & About',
        summary: 'Inspect Agora services and diagnostic tools.',
        icon: 'diagnostics',
        route: 'diagnostics',
        searchTerms: ['overlay', 'health', 'version'],
        capabilities: ['load', 'validate', 'apply', 'restore_defaults'],
        backendAdapter: 'settings-diagnostics',
        uiEntryPoint: 'settings-diagnostics',
        contractVersion: 1,
      },
      availability: { state: 'available' },
    },
  ],
};

export const diagnosticsSettingsStateFixture: DiagnosticsSettingsState = {
  moduleId: 'diagnostics',
  contractVersion: 1,
  revision: 7,
  active: { diagnosticOverlayEnabled: true },
  defaults: { diagnosticOverlayEnabled: false },
  service: {
    enabled: true,
    active: true,
    enabledState: 'enabled',
    activeState: 'active',
  },
  productVersion: 'settings-v1',
  settingsSchemaVersion: 1,
  components: [{ id: 'shell-gateway', label: 'Shell gateway', state: 'available', version: 'settings-v1', detail: 'active', recovery: 'Restart shell.' }],
  supportBundle: { schemaVersion: 1, generatedAtUnixMillis: 0, productVersion: 'settings-v1', settingsSchemaVersion: 1, components: [{ id: 'shell-gateway', label: 'Shell gateway', state: 'available', version: 'settings-v1', detail: 'active', recovery: 'Restart shell.' }] },
  availability: { state: 'available' },
};

export const diagnosticsApplyRequestFixture: DiagnosticsApplyRequest = {
  contractVersion: 1,
  baseRevision: diagnosticsSettingsStateFixture.revision,
  draft: { diagnosticOverlayEnabled: false },
};

export const surfaceLifecycleEvents: readonly SurfaceEvent[] = [
  { surfaceId: 'view-42', kind: 'mapped', ownerUid: 60001 },
  { surfaceId: 'view-42', kind: 'focused', ownerUid: 60001 },
  { surfaceId: 'view-42', kind: 'input_denied', ownerUid: 60001 },
];

export const surfaceLifecycleViewFixture = projectSurfaceLifecycle(surfaceLifecycleEvents);

export const surfaceLifecycleStateFixture = surfaceLifecycleState({
  ok: true,
  value: surfaceLifecycleEvents,
});

export const workSurfaceControlsViewModelFixture = workSurfaceControlsViewModel(
  dataState(surfaceLifecycleViewFixture),
);

export const catalogAppsRouteFixture: CatalogAppsResponse = {
  apps: [
    {
      id: 'example-browser',
      name: 'Example Browser',
      icon: 'example-browser',
    },
  ],
};

export const catalogAppsTransportResultFixture = decodeCatalogAppsResponse(catalogAppsRouteFixture);

export const appCatalogStateFixture = appCatalogState(catalogAppsTransportResultFixture);

export const appLauncherViewModelFixture = appLauncherViewModel(appCatalogStateFixture);

export const operatorEscalationsFixture: readonly AdminEscalationSummary[] = [
  {
    id: 'esc-7',
    requestedAtUnixMillis: 1000,
    actorUid: 60007,
    summary: 'Approve privileged browser capture',
  },
];

export const operatorAuditTailFixture: readonly AuditEvent[] = [
  {
    id: 'audit-1',
    actorUid: 60007,
    action: 'requested',
    subject: 'privileged browser capture',
    createdAtUnixMillis: 1000,
  },
];

export const operatorAgentHealthFixture: readonly AgentInfo[] = [
  { uid: 60001, displayName: 'agent-ready', state: 'ready' },
  { uid: 60002, displayName: 'agent-busy', state: 'busy' },
];

export const escalationsViewModelFixture = escalationsViewModel(
  dataState(operatorEscalationsFixture),
  2500,
);

export const auditTailViewModelFixture = auditTailViewModel(dataState(operatorAuditTailFixture));

export const agentHealthViewModelFixture = agentHealthViewModel(
  dataState(operatorAgentHealthFixture),
);

export interface ShellRenderClaimFixture {
  readonly scenario: string;
  readonly surface: string;
  readonly featureIds: readonly string[];
  readonly modelClaims: readonly string[];
}

export const shellRenderClaimFixtures: readonly ShellRenderClaimFixture[] = [
  {
    scenario: 'desktop-shell-surface-controls-model-fixture',
    surface: desktopShellComposition.surface,
    featureIds: desktopShellComposition.features.map((feature) => feature.id),
    modelClaims: [
      'surface-lifecycle-mapped-focused-input-denied',
      'work-surface-controls-summary',
      'app-launcher-catalog-visible',
    ],
  },
  {
    scenario: 'operator-console-boundary-projections-model-fixture',
    surface: operatorConsoleComposition.surface,
    featureIds: operatorConsoleComposition.features.map((feature) => feature.id),
    modelClaims: [
      'audit-tail-event-label',
      'escalation-pending-age',
      'agent-health-ready-busy-counts',
    ],
  },
];

export function assertSurfaceLifecycleFixture(): void {
  const [surface] = surfaceLifecycleViewFixture;
  if (!surface || surface.id !== 'view-42') {
    throw new Error('surface lifecycle fixture should track view-42');
  }
  if (!surface.mapped || !surface.focused) {
    throw new Error('surface lifecycle fixture should keep view-42 mapped and focused');
  }
  if (surface.ownerUid !== 60001 || surface.inputDeniedCount !== 1) {
    throw new Error('surface lifecycle fixture should preserve owner uid and denied input count');
  }

  if (workSurfaceControlsViewModelFixture.kind !== 'data') {
    throw new Error('work surface controls fixture should produce data state');
  }
  if (
    workSurfaceControlsViewModelFixture.value.surfaceCount !== 1 ||
    workSurfaceControlsViewModelFixture.value.focusedSurfaceId !== 'view-42' ||
    workSurfaceControlsViewModelFixture.value.deniedInputSurfaceIds[0] !== 'view-42'
  ) {
    throw new Error('work surface controls fixture should summarize view-42 lifecycle state');
  }
}

export function assertAppCatalogVerticalFixture(): void {
  if (catalogAppsPath !== '/api/catalog/apps') {
    throw new Error('catalog fixture should target the Go catalog route path');
  }
  if (!catalogAppsTransportResultFixture.ok) {
    throw new Error('catalog route fixture should decode through transport');
  }
  if (appCatalogStateFixture.kind !== 'data') {
    throw new Error('catalog state fixture should produce data state');
  }
  if (appLauncherViewModelFixture.kind !== 'data') {
    throw new Error('app launcher fixture should produce data state');
  }

  const [app] = appLauncherViewModelFixture.value.apps;
  if (!app || app.id !== 'example-browser') {
    throw new Error('app launcher fixture should expose example-browser');
  }
  if (app.label !== 'Example Browser' || app.icon !== 'example-browser') {
    throw new Error('app launcher fixture should preserve catalog label and icon');
  }
  if (appLauncherViewModelFixture.value.empty) {
    throw new Error('app launcher fixture should not be empty');
  }
}

export function assertOperatorFeatureFixtures(): void {
  if (escalationsViewModelFixture.kind !== 'data') {
    throw new Error('escalations fixture should produce data state');
  }
  const [escalation] = escalationsViewModelFixture.value.pending;
  if (!escalation || escalation.id !== 'esc-7' || escalation.ageMillis !== 1500) {
    throw new Error('escalations fixture should preserve pending escalation age');
  }

  if (auditTailViewModelFixture.kind !== 'data') {
    throw new Error('audit tail fixture should produce data state');
  }
  const [auditEvent] = auditTailViewModelFixture.value.events;
  if (!auditEvent || auditEvent.label !== 'requested privileged browser capture') {
    throw new Error('audit tail fixture should compose action and subject labels');
  }

  if (agentHealthViewModelFixture.kind !== 'data') {
    throw new Error('agent health fixture should produce data state');
  }
  if (
    agentHealthViewModelFixture.value.counts.ready !== 1 ||
    agentHealthViewModelFixture.value.counts.busy !== 1 ||
    agentHealthViewModelFixture.value.counts.offline !== 0 ||
    !agentHealthViewModelFixture.value.ready
  ) {
    throw new Error('agent health fixture should count ready and busy agents');
  }
}

export function assertShellRenderClaimFixtures(): void {
  const [desktop, operator] = shellRenderClaimFixtures;
  if (!desktop || desktop.scenario !== 'desktop-shell-surface-controls-model-fixture') {
    throw new Error('desktop shell fixture should expose stable evidence scenario');
  }
  if (
    !desktop.featureIds.includes('feature-work-surface-controls') ||
    !desktop.featureIds.includes('feature-app-launcher') ||
    !desktop.modelClaims.includes('surface-lifecycle-mapped-focused-input-denied')
  ) {
    throw new Error('desktop shell fixture should name surface and app launcher claims');
  }

  if (!operator || operator.scenario !== 'operator-console-boundary-projections-model-fixture') {
    throw new Error('operator console fixture should expose stable evidence scenario');
  }
  if (
    !operator.featureIds.includes('feature-audit-tail') ||
    !operator.featureIds.includes('feature-escalations') ||
    !operator.featureIds.includes('feature-agent-health') ||
    !operator.modelClaims.includes('agent-health-ready-busy-counts')
  ) {
    throw new Error('operator console fixture should name boundary projection claims');
  }
}

export function assertThemeFixture(): void {
  if (!evidenceThemeTokenNames.includes(shellThemeTokens.evidenceAccent)) {
    throw new Error('theme fixture should identify evidence accent separately from presentation accent');
  }
  if (evidenceThemeTokenNames.includes(shellThemeTokens.accent)) {
    throw new Error('presentation accent should remain changeable without being a visual evidence marker');
  }
  if (taskbarThemeVars.border !== 'var(--agora-accent)') {
    throw new Error('taskbar separator should follow the active presentation accent');
  }
  if (appLauncherThemeVars.itemBackground !== 'var(--agora-surface-raised)') {
    throw new Error('app launcher should consume shell theme surface tokens');
  }
  const liveClient = liveThemeClientScript();
  for (const marker of [
    '/api/settings/modules/appearance/load',
    'new BroadcastChannel("agora-de-theme")',
    'window.agoraTheme=api',
    '!evidenceTokens.has(name)',
    'window.setInterval(refresh,1500)',
  ]) {
    if (!liveClient.includes(marker)) {
      throw new Error(`live theme client missing ${marker}`);
    }
  }
}

export function assertSettingsContractFixtures(): void {
  const [diagnostics] = settingsCatalogFixture.modules;
  if (!diagnostics || diagnostics.manifest.id !== diagnosticsSettingsStateFixture.moduleId) {
    throw new Error('settings catalog and diagnostics state should share a stable module id');
  }
  if (diagnosticsApplyRequestFixture.baseRevision !== diagnosticsSettingsStateFixture.revision) {
    throw new Error('settings apply fixture should use the loaded authoritative revision');
  }
  if ('payload' in diagnosticsApplyRequestFixture) {
    throw new Error('settings module requests must not use a generic payload bag');
  }
}
