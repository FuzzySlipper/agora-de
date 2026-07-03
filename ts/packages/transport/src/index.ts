import type { ClassifiedError } from '@agora-de/protocol';

export function networkError(message: string): ClassifiedError {
  return { kind: 'network', message };
}

