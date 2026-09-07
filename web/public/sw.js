// Service worker: the app shell cache.
//
// A device that has opened the app once keeps working through a network drop. That is
// half of local-first; the other half is the score keeper's own log in IndexedDB, which
// is written before anything touches the network (docs/tech-stack.md §6).
//
// The API is never cached. A stale snapshot pretending to be live is worse than an
// obvious failure, and the score keeper client does not need one: it holds its own log.

const SHELL = 'porta-shell-v1';

self.addEventListener('install', (event) => {
  self.skipWaiting();
  event.waitUntil(caches.open(SHELL).then((cache) => cache.addAll(['/'])));
});

self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches
      .keys()
      .then((keys) => Promise.all(keys.filter((k) => k !== SHELL).map((k) => caches.delete(k))))
      .then(() => self.clients.claim()),
  );
});

self.addEventListener('fetch', (event) => {
  const request = event.request;
  if (request.method !== 'GET') return;

  const url = new URL(request.url);
  if (url.origin !== self.location.origin) return;
  if (url.pathname.startsWith('/api/')) return;

  // Hashed asset names come from Vite, so anything under /assets/ can be served from the
  // cache and only fetched once.
  if (url.pathname.startsWith('/assets/')) {
    event.respondWith(
      caches.match(request).then(
        (hit) =>
          hit ||
          fetch(request).then((response) => {
            const copy = response.clone();
            caches.open(SHELL).then((cache) => cache.put(request, copy));
            return response;
          }),
      ),
    );
    return;
  }

  // Every client-side route is the same document, so a navigation falls back to the
  // cached shell when the server is unreachable.
  if (request.mode === 'navigate') {
    event.respondWith(
      fetch(request)
        .then((response) => {
          const copy = response.clone();
          caches.open(SHELL).then((cache) => cache.put('/', copy));
          return response;
        })
        .catch(() => caches.match('/').then((hit) => hit || Response.error())),
    );
  }
});
