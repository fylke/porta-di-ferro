import { afterEach, describe, expect, it, vi } from 'vitest';
import { clientId, randomId } from './presence.svelte';

/**
 * The client id has to be makeable on an insecure origin. Every client but the
 * organizer's own browser is on http://<LAN address>, where browsers withhold
 * crypto.randomUUID(); an id generator that needed it took the score keeper client down
 * on every tablet at the venue and left the organizer's laptop working (issue #81).
 */
describe('the client id on a LAN address', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('is made without crypto.randomUUID', () => {
    // A browser on http://192.168.1.20: getRandomValues is there, randomUUID is not.
    vi.stubGlobal('crypto', {
      getRandomValues: (a: Uint8Array) => {
        for (let i = 0; i < a.length; i++) a[i] = i * 17;
        return a;
      },
    });
    expect(randomId()).toMatch(/^[0-9a-f]{16}$/);
    expect(clientId()).toMatch(/^[0-9a-f]{16}$/);
  });

  it('is made even without crypto at all', () => {
    vi.stubGlobal('crypto', undefined);
    expect(randomId()).toMatch(/^[0-9a-f]{16}$/);
  });

  it('is different each time it is made fresh', () => {
    expect(randomId()).not.toBe(randomId());
  });
});
