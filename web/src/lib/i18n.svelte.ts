/**
 * Swedish alongside English (design §7 item 11).
 *
 * The English text is the key: `t('Confirm exchange')` reads as what it says, and a key
 * with no Swedish entry comes out in English rather than as a placeholder. The Swedish
 * lives in sv.ts, and a test walks the source for every `t('...')` and fails if one has no
 * entry, so the dictionary cannot fall behind the screens.
 *
 * The choice is per client, not per server: a display and a score keeper client at the
 * same event may reasonably want different languages. It is remembered on the device,
 * and can be set from the address (`?lang=sv`) for a screen nobody will touch.
 *
 * Internal identifiers stay English whatever the language: the log, the API and the
 * files on disk say "red", "exchange" and "quarter", never their translations.
 */
import { sv } from './sv';

export type Lang = 'en' | 'sv';

const KEY = 'porta.lang';

function detect(): Lang {
  try {
    const fromUrl = new URLSearchParams(window.location.search).get('lang');
    if (fromUrl === 'sv' || fromUrl === 'en') {
      localStorage.setItem(KEY, fromUrl);
      return fromUrl;
    }
    const saved = localStorage.getItem(KEY);
    if (saved === 'sv' || saved === 'en') return saved;
    return navigator.language.toLowerCase().startsWith('sv') ? 'sv' : 'en';
  } catch {
    return 'en';
  }
}

/** The current language. Reactive: anything that reads it re-renders when it changes. */
export const lang = $state<{ current: Lang }>({ current: 'en' });
lang.current = detect();

export function setLang(next: Lang): void {
  lang.current = next;
  try {
    localStorage.setItem(KEY, next);
  } catch {
    // The choice holds for this load.
  }
  document.documentElement.lang = next;
}

/**
 * Translates a string, filling `{name}` placeholders from params. Reads the current
 * language, so a template expression that calls it follows the toggle.
 */
export function t(key: string, params?: Record<string, string | number>): string {
  let out = lang.current === 'sv' ? (sv[key] ?? key) : key;
  if (params) {
    for (const [k, v] of Object.entries(params)) out = out.replaceAll(`{${k}}`, String(v));
  }
  return out;
}

/** The locale for dates and times, matching the language. */
export function locale(): string {
  return lang.current === 'sv' ? 'sv-SE' : 'en-GB';
}
