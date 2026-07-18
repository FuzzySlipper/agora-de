import type { SettingsPageDefinition, SettingsPageRegistration } from '@agora-de/domain';
import type {
  SettingsCatalogEntry,
  SettingsCatalogResponse,
  SettingsError,
  SettingsModuleManifest,
} from '@agora-de/protocol';

export const settingsCatalogPath = '/api/settings/catalog';

export type SettingsLifecycleStatus =
  | 'loading'
  | 'ready'
  | 'validating'
  | 'applying'
  | 'unavailable'
  | 'error';

export interface SettingsDraftState<T> {
  readonly active?: T;
  readonly draft?: T;
  readonly revision?: number;
  readonly dirty: boolean;
  readonly status: SettingsLifecycleStatus;
  readonly error?: SettingsError;
}

export interface SettingsErrorPresentation {
  readonly title: string;
  readonly detail: string;
  readonly retryable: boolean;
}

export class SettingsPageRegistry {
  readonly #registrations = new Map<string, SettingsPageRegistration>();

  constructor(registrations: readonly SettingsPageRegistration[]) {
    for (const registration of registrations) {
      if (!registration.uiEntryPoint.trim()) {
        throw new Error('settings page registration needs a UI entry point');
      }
      if (this.#registrations.has(registration.uiEntryPoint)) {
        throw new Error(`duplicate settings UI entry point: ${registration.uiEntryPoint}`);
      }
      this.#registrations.set(registration.uiEntryPoint, registration);
    }
  }

  has(uiEntryPoint: string): boolean {
    return this.#registrations.has(uiEntryPoint);
  }

  async load(uiEntryPoint: string): Promise<SettingsPageDefinition> {
    const registration = this.#registrations.get(uiEntryPoint);
    if (!registration) {
      throw new Error(`settings UI entry point is not registered: ${uiEntryPoint}`);
    }
    const page = await registration.load();
    if (page.uiEntryPoint !== uiEntryPoint || !page.moduleId || !page.title) {
      throw new Error(`settings page ${uiEntryPoint} returned invalid metadata`);
    }
    return page;
  }
}

export function settingsModuleOperationPath(
  moduleId: string,
  operation: 'load' | 'validate' | 'apply' | 'reset' | 'restore_defaults' | 'keep' | 'revert',
): string {
  if (!/^[a-z0-9-]+$/.test(moduleId)) {
    throw new Error(`invalid settings module id: ${moduleId}`);
  }
  return `/api/settings/modules/${moduleId}/${operation}`;
}

export function filterSettingsCatalog(
  catalog: SettingsCatalogResponse,
  query: string,
): readonly SettingsCatalogEntry[] {
  const normalized = query.trim().toLocaleLowerCase();
  if (!normalized) return catalog.modules;
  return catalog.modules.filter(({ manifest }) =>
    [manifest.title, manifest.summary, manifest.category, ...manifest.searchTerms]
      .join(' ')
      .toLocaleLowerCase()
      .includes(normalized),
  );
}

export function resolveSettingsDeepLink(
  catalog: SettingsCatalogResponse,
  search: string,
): string | undefined {
  const requested = new URLSearchParams(search).get('module');
  if (requested && catalog.modules.some(({ manifest }) => manifest.id === requested)) {
    return requested;
  }
  return catalog.modules.find(({ availability }) => availability.state !== 'unsupported')?.manifest.id;
}

export function canNavigateSettings<T>(
  state: SettingsDraftState<T>,
  confirmDiscard: () => boolean,
): boolean {
  return !state.dirty || confirmDiscard();
}

export function loadingSettings<T>(): SettingsDraftState<T> {
  return { dirty: false, status: 'loading' };
}

export function loadedSettings<T>(active: T, revision: number): SettingsDraftState<T> {
  if (!Number.isSafeInteger(revision) || revision < 0) {
    throw new Error('settings revision must be a non-negative safe integer');
  }
  return { active, draft: active, revision, dirty: false, status: 'ready' };
}

export function editSettings<T>(state: SettingsDraftState<T>, draft: T): SettingsDraftState<T> {
  requireLoaded(state);
  return withoutError({ ...state, draft, dirty: true, status: 'ready' });
}

export function resetSettings<T>(state: SettingsDraftState<T>): SettingsDraftState<T> {
  requireLoaded(state);
  return withoutError({ ...state, draft: state.active, dirty: false, status: 'ready' });
}

export function restoreSettingsDefaults<T>(state: SettingsDraftState<T>, defaults: T): SettingsDraftState<T> {
  requireLoaded(state);
  return withoutError({ ...state, draft: defaults, dirty: true, status: 'ready' });
}

export function applyingSettings<T>(state: SettingsDraftState<T>): SettingsDraftState<T> {
  requireLoaded(state);
  return withoutError({ ...state, status: 'applying' });
}

export function settingsApplySucceeded<T>(active: T, revision: number): SettingsDraftState<T> {
  return loadedSettings(active, revision);
}

export function settingsFailed<T>(state: SettingsDraftState<T>, error: SettingsError): SettingsDraftState<T> {
  return { ...state, status: error.code === 'unavailable' ? 'unavailable' : 'error', error };
}

export function settingsErrorPresentation(error: SettingsError): SettingsErrorPresentation {
  const titles: Record<SettingsError['code'], string> = {
    invalid_request: 'The request was not understood',
    validation_failed: 'Some values need attention',
    stale_revision: 'Settings changed elsewhere',
    unsupported: 'This setting is not supported',
    unavailable: 'This settings service is unavailable',
    timeout: 'The settings service took too long',
    apply_failed: 'The change was not applied',
    rollback_failed: 'The previous state could not be restored',
    restart_required: 'A restart is required',
    test_failed: 'The compositor rejected this configuration',
    compositor_cancelled: 'The compositor cancelled this change',
    transaction_busy: 'Another display change is in progress',
    confirmation_expired: 'The confirmation period has ended',
  };
  return { title: titles[error.code], detail: error.message, retryable: error.retryable };
}

export function assertSettingsModuleContract(
  manifest: SettingsModuleManifest,
  registration: SettingsPageRegistration,
): void {
  if (!/^[a-z0-9-]+$/.test(manifest.id) || manifest.route !== manifest.id) {
    throw new Error('settings module id and route must be the same stable identifier');
  }
  if (!manifest.title.trim() || !manifest.summary.trim() || manifest.contractVersion <= 0) {
    throw new Error('settings module metadata and contract version are required');
  }
  if (!manifest.capabilities.includes('load')) {
    throw new Error('settings module must support load');
  }
  if (manifest.uiEntryPoint !== registration.uiEntryPoint) {
    throw new Error('manifest and UI registration entry points must match');
  }
}

function requireLoaded<T>(state: SettingsDraftState<T>): asserts state is SettingsDraftState<T> & {
  readonly active: T;
  readonly draft: T;
  readonly revision: number;
} {
  if (state.active === undefined || state.draft === undefined || state.revision === undefined) {
    throw new Error('settings lifecycle operation requires loaded active state and revision');
  }
}

function withoutError<T>(state: SettingsDraftState<T>): SettingsDraftState<T> {
  const { error: _error, ...rest } = state;
  return rest;
}
