/**
 * Whether this page is open on the organizer's own PC (#142).
 *
 * Nothing in the hall signs in: the organizer is whoever sits at the PC running the
 * server, which reaches it on loopback, while every phone and tablet comes in on the LAN
 * address. So a control meant only for the organizer -- Edit, on the landing page -- is
 * shown on loopback and nowhere else. Since #165 it is a permission too: the server takes
 * the organizer's changes from loopback alone, and an organizer page opened from the hall
 * says it can only look (LookOnly). The demo is a browser tab whose visitor is the
 * organizer of everything in it.
 */
export function onOrganizerPC(hostname = typeof location === 'undefined' ? '' : location.hostname): boolean {
  if (import.meta.env.VITE_DEMO === 'true') return true;
  return hostname === 'localhost' || hostname === '127.0.0.1' || hostname === '[::1]' || hostname === '::1';
}
