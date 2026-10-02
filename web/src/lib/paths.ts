/**
 * Addresses in an event with several disciplines (docs/proposals/one-event-many-
 * disciplines.md §6), as pure functions so they can be tested without a browser.
 *
 * Every discipline is at one address, so its pages carry it in the path --
 * /d/open-sabre/score/1 -- and its API is under /api/d/open-sabre/. Pages and API calls
 * with no discipline in them are the event's own, or, in an event with one discipline,
 * that discipline's: the paths every client used before there were several.
 */

const prefixed = /^\/d\/([^/?]+)(\/[^?]*)?/;
const adminPrefixed = /^\/admin\/([^/?]+)\/?$/;

/** The discipline an application path is about, or '' for none. */
export function disciplineOf(path: string): string {
  const p = path.split('?')[0];
  const m = p.match(prefixed) ?? p.match(adminPrefixed);
  if (!m) return '';
  try {
    return decodeURIComponent(m[1]);
  } catch {
    return m[1];
  }
}

/**
 * The route an application path is, with the discipline taken off, so a page is matched
 * the same way whichever discipline it is for. /admin/open-sabre is that discipline's
 * /admin.
 */
export function routeOf(path: string): string {
  const p = path.split('?')[0];
  if (adminPrefixed.test(p)) return '/admin';
  const m = p.match(prefixed);
  if (m) return m[2] && m[2] !== '/' ? m[2] : '/';
  return p;
}

/** A page of a discipline: /score/1 in Sabre is /d/open-sabre/score/1. */
export function within(slug: string, to: string): string {
  if (!slug || !to.startsWith('/')) return to;
  return `/d/${encodeURIComponent(slug)}${to}`;
}

/** Where a discipline's API is: its own prefix, or the unprefixed paths of a one-discipline event. */
export function apiBase(slug: string): string {
  return slug ? `/api/d/${encodeURIComponent(slug)}` : '/api';
}

/**
 * The storage namespace for what a device keeps about a discipline: '' for a page with no
 * discipline in its address, so a device that kept something before events existed still
 * finds it, and "open-sabre." otherwise, so two disciplines' "p1m1" never meet.
 */
export function namespace(slug: string): string {
  return slug ? `${slug}.` : '';
}
