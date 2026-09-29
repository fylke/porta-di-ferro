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

function broadcast(): void {
  if (streams.size === 0) return;
  const res = call('GET', '/api/state');
  if (res.status !== 200) return;
  const frame = JSON.stringify({ kind: 'state', data: JSON.parse(res.body) });
  for (const s of streams) s.push(frame);
}

/**
 * Enough of EventSource for the client: the three handlers it sets and close(). It is
 * not a general implementation and is not meant to be -- it stands in for exactly one
 * endpoint, /api/stream.
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

// --- installing the shims -------------------------------------------------------------

function installFetch(): void {
  window.fetch = async (input: RequestInfo | URL, init?: RequestInit): Promise<Response> => {
    const url = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url;
    const path = url.startsWith('http') ? new URL(url).pathname + new URL(url).search : url;
    if (!path.startsWith('/api/')) return realFetch(input as RequestInfo, init);

    const method = (init?.method ?? (typeof input === 'object' && 'method' in input ? input.method : 'GET')).toUpperCase();
    const body = typeof init?.body === 'string' ? init.body : undefined;
    const res = call(method, path, body);
    if (res.changed) broadcast();
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

      if (raw.startsWith('/api/export') || raw.startsWith('/api/signup/')) {
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
  reset(): void {
    call('POST', '/api/demo/reset');
    broadcast();
  },
  playRest(): void {
    call('POST', '/api/demo/play');
    broadcast();
  },
};
