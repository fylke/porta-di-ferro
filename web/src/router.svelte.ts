/**
 * A router, not a framework. Client-side routing is a small dependency here
 * (docs/tech-stack.md §6), and this is the small version of it: a reactive path, a
 * pattern matcher and a navigate function.
 *
 * The Go server falls through to index.html for anything it does not recognise, so every
 * route below is reachable by typing it in -- which is the whole point of the display
 * URLs being addressable (design §3).
 */

let current = $state(window.location.pathname + window.location.search);

window.addEventListener('popstate', () => {
  current = window.location.pathname + window.location.search;
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
  window.history.pushState({}, '', to);
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
