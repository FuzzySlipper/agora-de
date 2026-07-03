import type { ClassifiedError, DeResult } from '@agora-de/protocol';

export type AsyncState<T> =
  | { readonly kind: 'idle' }
  | { readonly kind: 'loading'; readonly previous?: T }
  | { readonly kind: 'data'; readonly value: T }
  | { readonly kind: 'error'; readonly error: ClassifiedError; readonly previous?: T };

export const idleState = <T>(): AsyncState<T> => ({ kind: 'idle' });
export const dataState = <T>(value: T): AsyncState<T> => ({ kind: 'data', value });

export function resultState<T>(result: DeResult<T>): AsyncState<T> {
  return result.ok ? dataState(result.value) : { kind: 'error', error: result.error };
}

