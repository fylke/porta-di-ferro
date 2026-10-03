import { describe, expect, it, vi, afterEach } from 'vitest';
import { Folds } from './folds.svelte';

describe('folded sections', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('are open until folded, and remembered per page', () => {
    const store = new Map<string, string>();
    vi.stubGlobal('localStorage', {
      getItem: (k: string) => store.get(k) ?? null,
      setItem: (k: string, v: string) => void store.set(k, v),
    });
    const a = new Folds('ls/landing');
    expect(a.open('pool-1')).toBe(true);
    a.toggle('pool-1');
    expect(a.open('pool-1')).toBe(false);
    expect(new Folds('ls/landing').open('pool-1')).toBe(false);
    expect(new Folds('sa/landing').open('pool-1')).toBe(true);
    a.toggle('pool-1');
    expect(new Folds('ls/landing').open('pool-1')).toBe(true);
  });

  it('work without storage', () => {
    vi.stubGlobal('localStorage', {
      getItem: () => {
        throw new Error('blocked');
      },
      setItem: () => {
        throw new Error('blocked');
      },
    });
    const f = new Folds('x');
    f.toggle('a');
    expect(f.open('a')).toBe(false);
  });
});
