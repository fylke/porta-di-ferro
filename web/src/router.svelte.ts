/**
 * A router, not a framework. Client-side routing is a small dependency here
 * (docs/tech-stack.md §6), and this is the small version of it: a reactive path, a
 * pattern matcher and a navigate function.
 *
 * The Go server falls through to index.html for anything it does not recognise, so every
 * route below is reachable by typing it in -- which is the whole point of the display
 * URLs being addressable (design §3).
 *
 * Everything here works in terms of application paths -- `/score/1` -- and not of the
 * URLs the browser shows. The two are the same thing when the application is served from
 * the root of a host, which is how an organizer's PC serves it and will always be the
 * deployment that matters. They are not the same on GitHub Pages, where the demo lives
 * under a project path (issue #88), so the base is stripped on the way in and put back on
 * the way out, in one place, rather than every route being written twice.
 */

/** "/" for the real application, "/porta-di-ferro/" for the demo on GitHub Pages. */
const base = import.meta.env.BASE_URL.replace(/\/+$/, '');

/** Strips the base, so a route pattern never has to know where the app is mounted. */
function appPath(url: string): string {
  if (base && url.startsWith(base)) return url.slice(base.length) || '/';
  return url;
}

/** Puts it back, for anything handed to the History API or to an href. */
export function href(to: string): string {
  return base && to.startsWith('/') ? base + to : to;
}

let current = $state(appPath(window.location.pathname) + window.location.search);

window.addEventListener('popstate', () => {
  current = appPath(window.location.pathname) + window.location.search;
});

export function path(): string {
  return current.split('?')[0];
}

export function query(): URLSearchParams {
  const q = current.split('?')[1] ?? '';
  return new URLSearchParams(q);
}

export function navigate(to: string): void {
  if (to === current) return;
  window.history.pushState({}, '', href(to));
  current = to;
}

/**
 * Matches a pattern like `/display/mat/:n` against the current path, returning the named
 * parts or null.
 */
export function route(pattern: string): Record<string, string> | null {
  const p = path().replace(/\/+$/, '') || '/';
  const want = pattern.replace(/\/+$/, '') || '/';
  const a = p.split('/');
  const b = want.split('/');
  if (a.length !== b.length) return null;
  const params: Record<string, string> = {};
  for (let i = 0; i < b.length; i++) {
    if (b[i].startsWith(':')) {
      params[b[i].slice(1)] = decodeURIComponent(a[i]);
      continue;
    }
    if (a[i] !== b[i]) return null;
  }
  return params;
}
