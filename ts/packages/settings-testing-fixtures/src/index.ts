import type { SettingsPageDefinition, SettingsPageRegistration } from '@agora-de/domain';
import {
  SettingsPageRegistry,
  applyingSettings,
  assertSettingsModuleContract,
  canNavigateSettings,
  editSettings,
  filterSettingsCatalog,
  loadedSettings,
  resetSettings,
  resolveSettingsDeepLink,
  restoreSettingsDefaults,
  settingsApplySucceeded,
  settingsErrorPresentation,
  settingsFailed,
  settingsModuleOperationPath,
} from '@agora-de/feature-settings-host';
import { assertDisplaySettingsFixtures } from '@agora-de/feature-settings-displays';
import { assertDiagnosticsFixtures } from '@agora-de/feature-settings-diagnostics';
import { assertWindowManagementFixtures } from '@agora-de/feature-settings-window-management';
import { assertAppearanceFixtures } from '@agora-de/feature-settings-appearance';
import { assertShortcutFixtures } from '@agora-de/feature-settings-shortcuts';
import type { SettingsCatalogResponse, SettingsError, SettingsModuleManifest } from '@agora-de/protocol';

let fixtureLoadCount = 0;

function currentFixtureLoadCount(): number {
  return fixtureLoadCount;
}

export const fixtureSettingsManifest: SettingsModuleManifest = {
  id: 'fixture-module',
  category: 'system',
  title: 'Fixture module',
  summary: 'Non-production module for authoring-kit contract tests.',
  icon: 'settings',
  route: 'fixture-module',
  searchTerms: ['fixture', 'lazy'],
  capabilities: ['load', 'validate', 'apply', 'restore_defaults'],
  backendAdapter: 'fixture-module',
  uiEntryPoint: 'fixture-module',
  contractVersion: 1,
};

const fixtureSettingsPage: SettingsPageDefinition = {
  moduleId: fixtureSettingsManifest.id,
  uiEntryPoint: fixtureSettingsManifest.uiEntryPoint,
  title: fixtureSettingsManifest.title,
  renderPanel: () => '<section data-settings-module="fixture-module">fixture</section>',
};

export const fixtureSettingsPageRegistration: SettingsPageRegistration = {
  uiEntryPoint: fixtureSettingsManifest.uiEntryPoint,
  load: async () => {
    fixtureLoadCount += 1;
    return fixtureSettingsPage;
  },
};

export const failingSettingsPageRegistration: SettingsPageRegistration = {
  uiEntryPoint: 'failing-fixture',
  load: async () => {
    throw new Error('fixture module load failed');
  },
};

export async function assertSettingsAuthoringKitFixtures(): Promise<void> {
  assertDisplaySettingsFixtures();
  assertDiagnosticsFixtures();
  assertWindowManagementFixtures();
  assertAppearanceFixtures();
  assertShortcutFixtures();
  assertSettingsModuleContract(fixtureSettingsManifest, fixtureSettingsPageRegistration);

  const catalog: SettingsCatalogResponse = {
    schemaVersion: 1,
    modules: [{ manifest: fixtureSettingsManifest, availability: { state: 'available' } }],
  };
  if (resolveSettingsDeepLink(catalog, '?module=fixture-module') !== 'fixture-module') {
    throw new Error('fixture deep link should resolve from the generated catalog');
  }
  if (filterSettingsCatalog(catalog, 'lazy').length !== 1) {
    throw new Error('fixture manifest search terms should be indexed');
  }
  const reorderedCatalog: SettingsCatalogResponse = {
    schemaVersion: 1,
    modules: [
      {
        manifest: {
          ...fixtureSettingsManifest,
          id: 'second-fixture',
          route: 'second-fixture',
          title: 'Second fixture',
          uiEntryPoint: 'second-fixture',
        },
        availability: { state: 'available' },
      },
      catalog.modules[0]!,
    ],
  };
  if (filterSettingsCatalog(reorderedCatalog, '').map(({ manifest }) => manifest.id).join(',') !== 'second-fixture,fixture-module') {
    throw new Error('catalog order must drive navigation without a host edit');
  }
  const removedCatalog: SettingsCatalogResponse = {
    ...reorderedCatalog,
    modules: reorderedCatalog.modules.filter(({ manifest }) => manifest.id !== 'fixture-module'),
  };
  if (filterSettingsCatalog(removedCatalog, '').some(({ manifest }) => manifest.id === 'fixture-module')) {
    throw new Error('removing a catalog module must remove it from navigation without a host edit');
  }
  if (settingsModuleOperationPath('fixture-module', 'apply') !== '/api/settings/modules/fixture-module/apply') {
    throw new Error('fixture operation path should follow the bounded module route');
  }
  if (settingsModuleOperationPath('fixture-module', 'keep') !== '/api/settings/modules/fixture-module/keep') {
    throw new Error('confirmation operations must use the bounded module route');
  }

  const registry = new SettingsPageRegistry([fixtureSettingsPageRegistration, failingSettingsPageRegistration]);
  if (currentFixtureLoadCount() !== 0) {
    throw new Error('fixture page must remain lazy before selection');
  }
  const page = await registry.load('fixture-module');
  if (page.moduleId !== 'fixture-module' || currentFixtureLoadCount() !== 1) {
    throw new Error('fixture page should load independently on selection');
  }
  let failureIsolated = false;
  try {
    await registry.load('failing-fixture');
  } catch {
    failureIsolated = true;
  }
  if (!failureIsolated || !(await registry.load('fixture-module'))) {
    throw new Error('one page load failure must not poison another module');
  }

  let state = loadedSettings({ enabled: false }, 7);
  state = editSettings(state, { enabled: true });
  if (!state.dirty || !state.draft?.enabled) {
    throw new Error('edit should create a dirty draft');
  }
  if (canNavigateSettings(state, () => false) || !canNavigateSettings(state, () => true)) {
    throw new Error('dirty navigation must require explicit discard confirmation');
  }
  state = resetSettings(state);
  if (state.dirty || state.draft?.enabled) {
    throw new Error('reset should restore active state without mutation');
  }
  state = restoreSettingsDefaults(state, { enabled: true });
  if (!state.dirty || !state.draft?.enabled || state.revision !== 7) {
    throw new Error('defaults should replace the draft and preserve the loaded revision');
  }
  state = applyingSettings(state);
  if (state.status !== 'applying') {
    throw new Error('apply transition should be explicit');
  }
  state = settingsApplySucceeded({ enabled: true }, 8);
  if (state.dirty || state.revision !== 8 || !state.active?.enabled) {
    throw new Error('apply success should replace active state and revision');
  }

  const unavailable: SettingsError = {
    code: 'unavailable',
    message: 'fixture backend unavailable',
    retryable: true,
    issues: [],
  };
  state = settingsFailed(state, unavailable);
  const presentation = settingsErrorPresentation(unavailable);
  if (state.status !== 'unavailable' || !presentation.retryable || !presentation.title) {
    throw new Error('unavailable error should preserve retry and rendering semantics');
  }

  const validationError: SettingsError = {
    code: 'validation_failed',
    message: 'fixture validation failed',
    retryable: false,
    issues: [{ field: 'enabled', code: 'invalid', message: 'enabled is invalid' }],
  };
  state = settingsFailed(state, validationError);
  if (state.status !== 'error' || settingsErrorPresentation(validationError).retryable) {
    throw new Error('validation errors must remain module-local and non-retryable');
  }

  const backendFailure: SettingsError = {
    code: 'apply_failed',
    message: 'fixture apply failed',
    retryable: true,
    issues: [],
  };
  state = settingsFailed(state, backendFailure);
  if (state.status !== 'error' || !settingsErrorPresentation(backendFailure).retryable) {
    throw new Error('backend apply failures must expose retry semantics without poisoning the host');
  }

  const invalidManifest: SettingsModuleManifest = { ...fixtureSettingsManifest, capabilities: [] };
  let contractRejected = false;
  try {
    assertSettingsModuleContract(invalidManifest, fixtureSettingsPageRegistration);
  } catch {
    contractRejected = true;
  }
  if (!contractRejected) {
    throw new Error('contract test must reject a module without load lifecycle support');
  }
}
