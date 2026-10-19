import { describe, expect, it } from 'vitest';
import { aboutTheEvent, verdict } from './forget';

describe('letting go of a deleted event (#141)', () => {
  it('forgets only when the event is another one than this device last saw', () => {
    expect(verdict('ev-1', 'ev-1')).toBe('same');
    expect(verdict('ev-1', 'ev-2')).toBe('other');
    // Never told an identity before: it cannot tell, so it keeps what it has.
    expect(verdict('', 'ev-2')).toBe('first');
    // A view with no identity -- an unreadable event.json -- is never another event.
    expect(verdict('ev-1', undefined)).toBe('same');
    expect(verdict('ev-1', '')).toBe('same');
  });

  it('forgets what is about the event and keeps what is about the device', () => {
    for (const key of [
      'porta.mat.2.current',
      'porta.rest.open-sabre.p1m1',
      'porta.rest.p1m1',
      'porta.mats',
      'porta.event',
      'porta.person.pr-1',
      'porta.snapshot',
      'porta.open-sabre.snapshot',
    ])
      expect(aboutTheEvent(key), key).toBe(true);
    for (const key of [
      'porta.clientId',
      'porta.lang',
      'porta.clientAddress',
      'porta.mat.2.swap',
      'porta.folds.admin',
      'porta.demo.banner',
      'porta.eventId',
      'porta-di-ferro-demo',
    ])
      expect(aboutTheEvent(key), key).toBe(false);
  });
});
