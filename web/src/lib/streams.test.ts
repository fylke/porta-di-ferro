import { afterEach, describe, expect, it, vi } from 'vitest';

/** Enough of EventSource to see what is open, and to feed it frames. */
class FakeSource {
  static all: FakeSource[] = [];
  readyState = 0;
  onopen: (() => void) | null = null;
  onmessage: ((ev: { data: string }) => void) | null = null;
  onerror: (() => void) | null = null;
  constructor(public url: string) {
    FakeSource.all.push(this);
  }
  close() {
    this.readyState = 2;
  }
  static get open() {
    return FakeSource.all.filter((s) => s.readyState !== 2);
  }
}

const settled = () => new Promise<void>((r) => setTimeout(r, 0));

describe('one stream per page (#161)', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.resetModules();
    FakeSource.all = [];
  });

  // REQ: DC_TRANSPORT_01
  it('carries every topic a page follows over one connection, and hands each its own', async () => {
    vi.stubGlobal('EventSource', FakeSource);
    const { follow } = await import('./streams');
    const got: string[] = [];
    const opened: string[] = [];
    const stopEvent = follow('event', { onMessage: (u) => got.push(`event:${u.kind}`), onOpen: () => opened.push('event') });
    const stopMats = follow('mats', { onMessage: (u) => got.push(`mats:${u.kind}`) });
    const stopSabre = follow('d/open-sabre', { onMessage: (u) => got.push(`sabre:${u.kind}`) });
    await settled();
    expect(FakeSource.open).toHaveLength(1);
    const s = FakeSource.open[0];
    expect(s.url).toBe('/api/streams?topic=d%2Fopen-sabre&topic=event&topic=mats');

    s.readyState = 1;
    s.onopen?.();
    s.onmessage?.({ data: JSON.stringify({ topic: 'mats', kind: 'mats' }) });
    s.onmessage?.({ data: JSON.stringify({ topic: 'd/open-sabre', kind: 'state' }) });
    s.onmessage?.({ data: 'not json' });
    expect(opened).toEqual(['event']);
    expect(got).toEqual(['mats:mats', 'sabre:state']);

    // A page leaving one topic reopens the connection without it; the last leaving closes it.
    stopSabre();
    await settled();
    expect(FakeSource.open).toHaveLength(1);
    expect(FakeSource.open[0].url).toBe('/api/streams?topic=event&topic=mats');
    stopEvent();
    stopMats();
    await settled();
    expect(FakeSource.open).toHaveLength(0);
  });

  it('lets go of the connection while the tab is hidden, and asks again when it is back', async () => {
    vi.useFakeTimers();
    vi.stubGlobal('EventSource', FakeSource);
    // The tests run in Node: a document is only its visibility and its events.
    let state = 'visible';
    const doc = new EventTarget();
    Object.defineProperty(doc, 'visibilityState', { get: () => state });
    vi.stubGlobal('document', doc);
    const { follow } = await import('./streams');
    let opens = 0;
    follow('mats', { onMessage: () => {}, onOpen: () => opens++ });
    await vi.advanceTimersByTimeAsync(0);
    expect(FakeSource.open).toHaveLength(1);

    state = 'hidden';
    doc.dispatchEvent(new Event('visibilitychange'));
    await vi.advanceTimersByTimeAsync(5_000);
    expect(FakeSource.open).toHaveLength(1); // a quick look at another tab keeps it
    await vi.advanceTimersByTimeAsync(15_000);
    expect(FakeSource.open).toHaveLength(0);

    state = 'visible';
    doc.dispatchEvent(new Event('visibilitychange'));
    expect(FakeSource.open).toHaveLength(1);
    FakeSource.open[0].onopen?.();
    expect(opens).toBe(1);
    vi.useRealTimers();
  });
});
