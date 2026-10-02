import { afterEach, describe, expect, it, vi } from 'vitest';
import { EventLive } from './event.svelte';

/** Enough of EventSource to count what is open. */
class FakeSource {
  static open = 0;
  onopen: (() => void) | null = null;
  onmessage: ((ev: { data: string }) => void) | null = null;
  onerror: (() => void) | null = null;
  constructor(public url: string) {
    FakeSource.open++;
  }
  close() {
    FakeSource.open--;
  }
}

describe('following the event', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('keeps the stream open while any page still follows it', () => {
    vi.stubGlobal('EventSource', FakeSource);
    const hall = new EventLive();
    // The landing page hands over to a person's page: the new one follows before the old
    // one lets go.
    hall.follow();
    hall.follow();
    expect(FakeSource.open).toBe(1);
    hall.unfollow();
    expect(FakeSource.open).toBe(1);
    hall.unfollow();
    expect(FakeSource.open).toBe(0);
    // One too many is harmless.
    hall.unfollow();
    hall.follow();
    expect(FakeSource.open).toBe(1);
    hall.unfollow();
    expect(FakeSource.open).toBe(0);
  });
});
