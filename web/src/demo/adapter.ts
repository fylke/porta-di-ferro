/**
 * The demo adapter: the Go server, replaced by a WebAssembly module in the same tab
 * (issue #88).
 *
 * The rule this is built to is that the application must not know it is in a demo. Every
 * screen, every request and every reconnect is the real client's; what changes is only
 * what is on the other end of `fetch` and `EventSource`. That is why this is a pair of
 * shims installed before the app mounts rather than a set of `if (demo)` branches through
 * api.ts and live.svelte.ts: branches would have to be kept in step with every change to
 * the real paths, and the first one anybody forgot would make the demo quietly lie.
 *
 * The module on the other side is cmd/demo-wasm, which is internal/tournament,
 * internal/match and httpapi.BuildSnapshot -- the same code the server runs. So the
 * standings, the tie-break chain and the bracket in the demo are not an approximation of
 * a real event's; they are the same computation over a tournament that happens to live in
 * memory. See docs/demo.md.
 */

/** What cmd/demo-wasm hands back. Mirrors demo.Response. */
interface DemoResponse {
  status: number;
  contentType: string;
  body: string;
  base64?: boolean;
  /** The call changed the tournament, so every open stream needs a new snapshot. */
  changed?: boolean;
}

declare global {
  interface Window {
    portaDemo?: { request: (method: string, path: string, body?: string) => string };
    /** Set by the demo only. See portaQR below. */
    portaQR?: (payload: string) => string;
    __portaDemoReady?: () => void;
    Go: new () => { importObject: WebAssembly.Imports; run: (i: WebAssembly.Instance) => void };
  }
}

const realFetch = window.fetch.bind(window);

function call(method: string, path: string, body?: string): DemoResponse {
  if (!window.portaDemo) {
    return { status: 503, contentType: 'application/json', body: '{"error":"the demo is still loading"}' };
  }
  return JSON.parse(window.portaDemo.request(method, path, body)) as DemoResponse;
}

/**
 * Base64 to bytes, for the one response that is not text: the printable pool sheets.
 *
 * Typed as its own ArrayBuffer rather than the ArrayBufferLike a bare Uint8Array carries,
 * because Response and Blob both refuse anything that might be a SharedArrayBuffer.
 */
function bytes(b64: string): Uint8Array<ArrayBuffer> {
  const raw = atob(b64);
  const out = new Uint8Array(new ArrayBuffer(raw.length));
  for (let i = 0; i < raw.length; i++) out[i] = raw.charCodeAt(i);
  return out;
}

function toResponse(res: DemoResponse): Response {
  const body = res.base64 ? bytes(res.body) : res.body;
  return new Response(body, {
    status: res.status,
    headers: { 'Content-Type': res.contentType },
  });
}

// --- the stand-in for the event stream ------------------------------------------------
//
// The server pushes a new snapshot whenever anything changes. Here, "whenever anything
// changes" is known exactly: it is any request the module reports as having changed the
// tournament. So the streams are fed from the write path rather than polled, which is
// both cheaper and a closer match to what the client is written against.

const streams = new Set<DemoEventSource>();

/**
 * What a stream at this URL is sent: the event's view on /api/event/stream, the hall's
 * mats on /api/mats/stream, and a
 * discipline's snapshot on its own stream -- /api/d/{slug}/stream, or /api/stream while
 * the event has one discipline. The same frames the server's hubs push.
 */
function frameFor(url: string): string | null {
  const path = url.startsWith('http') ? new URL(url).pathname : url;
  if (path.endsWith('/api/event/stream')) {
    const res = call('GET', '/api/event');
    return res.status === 200 ? JSON.stringify({ kind: 'event', data: JSON.parse(res.body) }) : null;
  }
  if (path.endsWith('/api/mats/stream')) {
    const res = call('GET', '/api/mats');
    return res.status === 200 ? JSON.stringify({ kind: 'mats', data: JSON.parse(res.body) }) : null;
  }
  const res = call('GET', path.replace(/\/stream$/, '/state'));
  return res.status === 200 ? JSON.stringify({ kind: 'state', data: JSON.parse(res.body) }) : null;
}

function broadcast(): void {
  // Several pages in one tab can follow the same stream; each frame is built once.
  const frames = new Map<string, string | null>();
  for (const s of streams) {
    if (!frames.has(s.url)) frames.set(s.url, frameFor(s.url));
    const frame = frames.get(s.url);
    if (frame) s.push(frame);
  }
}

/**
 * Enough of EventSource for the client: the three handlers it sets and close(). It is
 * not a general implementation and is not meant to be -- it stands in for the streams the
 * server has, the event's and each discipline's.
 */
class DemoEventSource extends EventTarget {
  onopen: ((ev: Event) => void) | null = null;
  onmessage: ((ev: MessageEvent) => void) | null = null;
  onerror: ((ev: Event) => void) | null = null;
  readyState = 0;
  readonly url: string;
  readonly withCredentials = false;
  static readonly CONNECTING = 0;
  static readonly OPEN = 1;
  static readonly CLOSED = 2;

  constructor(url: string) {
    super();
    this.url = url;
    streams.add(this);
    // Asynchronously, like a real connection: a handler assigned on the line after the
    // constructor still has to see the open.
    setTimeout(() => {
      if (this.readyState === 2) return;
      this.readyState = 1;
      this.onopen?.(new Event('open'));
    }, 0);
  }

  push(data: string): void {
    if (this.readyState === 2) return;
    this.onmessage?.(new MessageEvent('message', { data }));
  }

  close(): void {
    this.readyState = 2;
    streams.delete(this);
  }
}

// --- keeping the tournament between tabs (issue #108) ---------------------------------
//
// Every tab runs its own copy of the module, and several of the organizer's links open a
// new tab, so a visitor who edited the welcome message and opened the landing page found
// the demo as it was before they touched it. The tournament is therefore kept in
// localStorage after every change, taken up by each tab as it opens, and followed by the
// tabs already open -- which makes the score keeper in one tab and the organizer's view
// in another the same tournament, the way the tablets and the PC are at an event.
//
// It is still one browser. Nothing leaves it, and a visitor who comes back after a while
// away finds the demo as everyone first finds it, not as they or somebody on the same
// computer left it.

const SAVED = 'porta-di-ferro-demo';
/** When the visitor last did anything. Its own key, so touching it is not a whole save. */
const SEEN = 'porta-di-ferro-demo-seen';
/** How long the demo waits for a visitor before it starts over. */
const IDLE_MS = 30 * 60 * 1000;

let lastSeen = 0;

/** Marks the visitor as still here, at most every few seconds. */
function seen(): void {
  const now = Date.now();
  if (now - lastSeen < 5000) return;
  lastSeen = now;
  try {
    localStorage.setItem(SEEN, String(now));
  } catch {
    // Storage blocked or full: the demo works, it just will not follow into a new tab.
  }
}

/** Writes the whole tournament out, after anything that changed it. */
function persist(): void {
  const res = call('GET', '/api/demo/save');
  if (res.status !== 200) return;
  try {
    localStorage.setItem(SAVED, res.body);
    lastSeen = 0;
    seen();
  } catch {
    // As above.
  }
}

function forget(): void {
  try {
    localStorage.removeItem(SAVED);
    localStorage.removeItem(SEEN);
  } catch {
    // As above.
  }
}

/**
 * Takes up the tournament another tab left, if the visitor was here recently. A save
 * the module will not take -- an older demo's, say -- is dropped, and the tab starts
 * from the fixture like a first visit.
 */
function restore(): void {
  let state: string | null = null;
  let at = 0;
  try {
    state = localStorage.getItem(SAVED);
    at = Number(localStorage.getItem(SEEN) ?? 0);
  } catch {
    return;
  }
  if (state === null) return;
  if (!(Date.now() - at < IDLE_MS)) {
    forget();
    return;
  }
  if (call('POST', '/api/demo/load', state).status !== 200) forget();
}

/** Follows the other tabs: their save is this tab's tournament too. */
function follow(): void {
  window.addEventListener('storage', (ev) => {
    if (ev.key !== SAVED && ev.key !== null) return;
    if (ev.newValue === null) call('POST', '/api/demo/reset');
    else call('POST', '/api/demo/load', ev.newValue);
    broadcast();
  });
}

// --- installing the shims -------------------------------------------------------------

function installFetch(): void {
  window.fetch = async (input: RequestInfo | URL, init?: RequestInit): Promise<Response> => {
    const url = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url;
    const path = url.startsWith('http') ? new URL(url).pathname + new URL(url).search : url;
    if (!path.startsWith('/api/')) return realFetch(input as RequestInfo, init);

    const method = (init?.method ?? (typeof input === 'object' && 'method' in input ? input.method : 'GET')).toUpperCase();
    const body = typeof init?.body === 'string' ? init.body : undefined;
    const res = call(method, path, body);
    seen();
    if (res.changed) {
      broadcast();
      persist();
    }
    return toResponse(res);
  };
}

/**
 * Two links in the organizer view are browser navigations rather than fetches -- the JSON
 * and PDF exports -- so nothing above would catch them, and on GitHub Pages they would
 * be a 404. They are turned into downloads of what the module produces, which is the
 * same document the server would have sent.
 *
 * The same handler keeps every other in-app link inside the demo. The application writes
 * `href="/display/roster"`, which under a project path would leave the site entirely.
 */
function installLinks(navigate: (to: string) => void): void {
  document.addEventListener(
    'click',
    (ev) => {
      if (ev.defaultPrevented || ev.button !== 0 || ev.metaKey || ev.ctrlKey || ev.shiftKey) return;
      const anchor = (ev.target as HTMLElement | null)?.closest?.('a');
      if (!anchor) return;
      const raw = anchor.getAttribute('href');
      if (!raw || !raw.startsWith('/')) return;

      // A discipline's files are under its prefix in an event of several (#102).
      const bare = raw.replace(/^\/api\/d\/[^/]+/, '/api');
      if (bare.startsWith('/api/export') || bare.startsWith('/api/signup/') || bare.startsWith('/api/event/signup/')) {
        ev.preventDefault();
        const res = call('GET', raw);
        if (res.status !== 200) return;
        const blob = new Blob([res.base64 ? bytes(res.body) : res.body], { type: res.contentType });
        const href = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = href;
        a.download = downloadName(raw, res.contentType);
        a.click();
        URL.revokeObjectURL(href);
        return;
      }
      if (raw.startsWith('/api/')) return;

      // A link meant for a new tab stays a new tab, with the base put back on so it
      // lands inside the demo.
      if (anchor.target === '_blank') {
        ev.preventDefault();
        window.open(import.meta.env.BASE_URL.replace(/\/+$/, '') + raw, '_blank', 'noreferrer');
        return;
      }
      ev.preventDefault();
      navigate(raw);
    },
    true,
  );
}

/** What a downloaded file should be called, from the path that produced it. */
function downloadName(path: string, contentType: string): string {
  if (path.includes('/signup/')) {
    return contentType.startsWith('text/html') ? 'signup-demo.html' : 'signup-demo.json';
  }
  return path.includes('.pdf') ? 'porta-di-ferro.pdf' : 'porta-di-ferro.json';
}

/** Loads cmd/demo-wasm and puts the shims in place. Resolves once it can answer. */
export async function startDemo(navigate: (to: string) => void): Promise<void> {
  const base = import.meta.env.BASE_URL;

  await new Promise<void>((resolve, reject) => {
    const script = document.createElement('script');
    script.src = `${base}wasm_exec.js`;
    script.onload = () => resolve();
    script.onerror = () => reject(new Error('could not load the Go WebAssembly runtime'));
    document.head.appendChild(script);
  });

  const go = new window.Go();
  const ready = new Promise<void>((resolve) => {
    window.__portaDemoReady = resolve;
  });

  // GitHub Pages serves .wasm as application/wasm, so this streams and compiles as it
  // downloads rather than waiting for the whole three megabytes. The fallback is for
  // anywhere that gets the content type wrong.
  const response = await realFetch(`${base}demo.wasm`);
  let result: WebAssembly.WebAssemblyInstantiatedSource;
  try {
    result = await WebAssembly.instantiateStreaming(response.clone(), go.importObject);
  } catch {
    result = await WebAssembly.instantiate(await response.arrayBuffer(), go.importObject);
  }
  void go.run(result.instance);
  await ready;

  restore();
  follow();
  installFetch();
  window.EventSource = DemoEventSource as unknown as typeof EventSource;
  installLinks(navigate);

  /**
   * A QR code as a data URL.
   *
   * The info sheet asks for its codes with <img src="/api/qr.png?...">, and an image load
   * does not go through fetch -- so the shim above never sees it and the demo showed two
   * broken images on a page that is mostly two codes. A global rather than an import,
   * because the page that wants it must not pull the demo into the real bundle: there it
   * is undefined behind a constant Rollup removes.
   */
  window.portaQR = (payload: string) => {
    const res = call('GET', `/api/qr.png?url=${encodeURIComponent(payload)}`);
    if (res.status !== 200) return '';
    return `data:${res.contentType};base64,${res.body}`;
  };
}

/** The demo's own controls, for the banner. Both go through the module like anything else. */
export const demoControls = {
  /** Start over is for every tab: the others hear the save go and reset with it. */
  reset(): void {
    call('POST', '/api/demo/reset');
    forget();
    broadcast();
  },
  playRest(): void {
    call('POST', '/api/demo/play');
    broadcast();
    persist();
  },
};
