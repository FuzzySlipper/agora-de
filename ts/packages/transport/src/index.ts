import type { CatalogApp, CatalogAppsResponse, ClassifiedError, DeResult } from '@agora-de/protocol';

export const catalogAppsPath = '/api/catalog/apps';

export function networkError(message: string): ClassifiedError {
  return { kind: 'network', message };
}

export function invalidResponse(message: string): ClassifiedError {
  return { kind: 'invalid-response', message };
}

export function decodeCatalogAppsResponse(value: unknown): DeResult<readonly CatalogApp[]> {
  if (!isRecord(value) || !Array.isArray(value.apps)) {
    return { ok: false, error: invalidResponse('catalog apps response must contain apps array') };
  }

  const apps: CatalogApp[] = [];
  for (const item of value.apps) {
    if (!isCatalogApp(item)) {
      return { ok: false, error: invalidResponse('catalog app entry has invalid shape') };
    }
    apps.push(item);
  }

  return { ok: true, value: apps };
}

export function catalogAppsResponse(apps: readonly CatalogApp[]): CatalogAppsResponse {
  return { apps };
}

function isCatalogApp(value: unknown): value is CatalogApp {
  return (
    isRecord(value) &&
    typeof value.id === 'string' &&
    typeof value.name === 'string' &&
    typeof value.icon === 'string'
  );
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}
