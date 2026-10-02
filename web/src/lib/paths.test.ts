import { describe, expect, it } from 'vitest';
import { apiBase, disciplineOf, namespace, routeOf, within } from './paths';

describe('addresses in an event', () => {
  it('reads the discipline from a page of it', () => {
    expect(disciplineOf('/d/open-sabre/score/1')).toBe('open-sabre');
    expect(disciplineOf('/d/open-sabre')).toBe('open-sabre');
    expect(disciplineOf('/d/open-sabre/?lang=sv')).toBe('open-sabre');
    expect(disciplineOf('/admin/open-sabre')).toBe('open-sabre');
  });

  it('finds none on the event pages and the unprefixed ones', () => {
    for (const p of ['/', '/admin', '/info', '/score/1', '/display/mats?ids=1,2', '/display/d/x']) {
      expect(disciplineOf(p)).toBe('');
    }
  });

  it('matches a page the same way whichever discipline it is for', () => {
    expect(routeOf('/d/open-sabre/score/1')).toBe('/score/1');
    expect(routeOf('/score/1')).toBe('/score/1');
    expect(routeOf('/d/open-sabre')).toBe('/');
    expect(routeOf('/d/open-sabre/')).toBe('/');
    expect(routeOf('/d/open-sabre/who/c7?x=1')).toBe('/who/c7');
    expect(routeOf('/admin/open-sabre')).toBe('/admin');
    expect(routeOf('/admin')).toBe('/admin');
  });

  it('keeps links within the discipline', () => {
    expect(within('open-sabre', '/score/1')).toBe('/d/open-sabre/score/1');
    expect(within('open-sabre', '/')).toBe('/d/open-sabre/');
    expect(within('', '/score/1')).toBe('/score/1');
    expect(within('open-sabre', 'https://example.org')).toBe('https://example.org');
  });

  it('puts the API where the server mounts it', () => {
    expect(apiBase('open-sabre')).toBe('/api/d/open-sabre');
    expect(apiBase('')).toBe('/api');
  });

  // What a device kept before events existed has to be found under the same key, and two
  // disciplines' p1m1 must never share one.
  it('namespaces storage without moving what was there', () => {
    expect(`porta.rest.${namespace('')}p1m1`).toBe('porta.rest.p1m1');
    expect(`porta.rest.${namespace('open-sabre')}p1m1`).toBe('porta.rest.open-sabre.p1m1');
  });
});
