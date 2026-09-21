import { describe, expect, it } from 'vitest';
import { readFileSync, readdirSync, statSync } from 'node:fs';
import { join } from 'node:path';
import { sv } from './sv';
import { lang, t } from './i18n.svelte';

/**
 * The dictionary cannot fall behind the screens: every `t('...')` in the source needs a
 * Swedish entry, and every entry keeps its placeholders. English needs no entries, because
 * the English is the key.
 */
const root = join(import.meta.dirname, '..');

function walk(dir: string, out: string[] = []): string[] {
  for (const name of readdirSync(dir)) {
    const p = join(dir, name);
    if (statSync(p).isDirectory()) walk(p, out);
    else if ((p.endsWith('.svelte') || p.endsWith('.ts')) && !p.endsWith('.test.ts') && !p.endsWith('sv.ts') && !p.endsWith('i18n.svelte.ts')) {
      out.push(p);
    }
  }
  return out;
}

function keysInSource(): string[] {
  const keys = new Set<string>();
  const pattern = /\bt\(\s*(?:'((?:[^'\\]|\\.)*)'|"((?:[^"\\]|\\.)*)")/g;
  for (const file of walk(root)) {
    const src = readFileSync(file, 'utf8');
    for (const m of src.matchAll(pattern)) keys.add((m[1] ?? m[2]).replace(/\\'/g, "'"));
  }
  return [...keys].sort();
}

// Keys reached through a variable rather than a literal: colours, sides, event types,
// timer actions, end reasons and pending states as the editor shows them.
const dynamic = [
  'red', 'blue', 'green', 'yellow', 'orange', 'purple', 'white',
  'exchange', 'timer', 'undo', 'end', 'options',
  'start', 'stop', 'resume', 'reset',
  'time', 'point cap', 'penalty', 'forfeit', 'final exchange', 'penalty cap',
];

describe('Swedish', () => {
  it('has an entry for every key the source uses', () => {
    const missing = [...keysInSource(), ...dynamic].filter((k) => !(k in sv));
    expect(missing).toEqual([]);
  });

  it('keeps every placeholder', () => {
    const broken: string[] = [];
    for (const [en, se] of Object.entries(sv)) {
      const want = [...en.matchAll(/\{(\w+)\}/g)].map((m) => m[1]).sort();
      const have = [...se.matchAll(/\{(\w+)\}/g)].map((m) => m[1]).sort();
      if (want.join() !== have.join()) broken.push(en);
    }
    expect(broken).toEqual([]);
  });

  it('has no entry nothing uses', () => {
    const used = new Set([...keysInSource(), ...dynamic]);
    const stale = Object.keys(sv).filter((k) => !used.has(k));
    expect(stale).toEqual([]);
  });

  it('translates and falls back', () => {
    lang.current = 'sv';
    expect(t('CONFIRM EXCHANGE')).toBe('BEKRÄFTA UTVÄXLING');
    expect(t('Mat {n}', { n: 2 })).toBe('Matta 2');
    expect(t('not in the dictionary')).toBe('not in the dictionary');
    lang.current = 'en';
    expect(t('Mat {n}', { n: 2 })).toBe('Mat 2');
  });
});
