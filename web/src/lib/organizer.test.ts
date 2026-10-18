import { describe, expect, it } from 'vitest';
import { onOrganizerPC } from './organizer';

describe('the organizer’s PC (#142)', () => {
  it('is loopback, and no LAN address', () => {
    expect(onOrganizerPC('localhost')).toBe(true);
    expect(onOrganizerPC('127.0.0.1')).toBe(true);
    expect(onOrganizerPC('[::1]')).toBe(true);
    expect(onOrganizerPC('192.168.1.20')).toBe(false);
    expect(onOrganizerPC('porta.local')).toBe(false);
  });
});
