import { describe, expect, it } from 'vitest';
import { readFileSync } from 'node:fs';
import { accept, kindOf, parseSv, standing, usages } from '../../scripts/i18n-review.mjs';

// The translation review tool (#168).
describe('the translation review tool', () => {
  const source = `export const sv: Record<string, string> = {
  // --- colours ------------------------------------------------------
  red: 'röd',
  'the {side} side': '{side} sida',

  // --- the mat board (phase 2) ----------------------------------------
  'Start':
    'Starta',
};`;

  it('reads sv.ts with its sections', () => {
    expect(parseSv(source)).toEqual([
      { key: 'red', sv: 'röd', section: 'colours', line: 3 },
      { key: 'the {side} side', sv: '{side} sida', section: 'colours', line: 4 },
      { key: 'Start', sv: 'Starta', section: 'the mat board (phase 2)', line: 7 },
    ]);
  });

  it('reads the real sv.ts, every entry of it', () => {
    const text = readFileSync(new URL('./sv.ts', import.meta.url), 'utf8');
    const entries = parseSv(text);
    // Every line that starts an entry: a quoted key, either quote, or a bare one ("Linköping").
    const literal = text.split('\n').filter((l) => /^\s{2}(['"]|\p{L}+:)/u.test(l)).length;
    expect(entries.length).toBe(literal);
    expect(entries.every((e) => e.section !== '')).toBe(true);
  });

  it('finds where each string is used, and what it is there', () => {
    const files = [
      { path: 'routes/MatBoard.svelte', text: `<button onclick={go}>{t('Start')}</button>\n<input placeholder={t('red')} />` },
      { path: 'lib/x.ts', text: `const s = t("the {side} side", { side });` },
    ];
    expect(usages(files, ['Start', 'red', 'the {side} side', 'unused'])).toEqual({
      Start: { files: ['MatBoard.svelte'], kinds: ['button'] },
      red: { files: ['MatBoard.svelte'], kinds: ['placeholder'] },
      'the {side} side': { files: ['x.ts'], kinds: ['text'] },
    });
    expect(kindOf('<h2>{t("Mats")}</h2>')).toBe('heading');
  });

  it('says what is new or changed since the last review, and takes a review back', () => {
    const entries = parseSv(source);
    expect(standing(entries, { red: 'röd', Start: 'Börja' })).toEqual({
      red: 'reviewed',
      'the {side} side': 'new',
      Start: 'changed',
    });
    const { reviewed, suggestions } = accept(
      { red: 'röd' },
      {
        entries: {
          'the {side} side': { sv: '{side} sida', approved: true },
          Start: { sv: 'Starta', approved: false, suggestion: 'Kör igång', note: 'ledigare' },
          red: { sv: 'gammal', approved: true },
        },
      },
      entries,
    );
    expect(reviewed).toEqual({ red: 'röd', 'the {side} side': '{side} sida' });
    expect(suggestions).toEqual([{ key: 'Start', line: 7, from: 'Starta', to: 'Kör igång', note: 'ledigare' }]);
  });
});
