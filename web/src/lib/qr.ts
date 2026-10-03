/**
 * QR codes and the address they point at, for every page that prints or shows one.
 *
 * The server renders the codes; the demo has no server, and an <img> load does not go
 * through the shim that stands in for one, so there the module hands back a data URL.
 * VITE_DEMO is a build-time constant, so the demo's branches are gone from the real bundle.
 */
const demo = import.meta.env.VITE_DEMO === 'true';

/** The image source for a QR code of payload. */
export function qrSrc(payload: string): string {
  return demo && window.portaQR ? window.portaQR(payload) : `/api/qr.png?url=${encodeURIComponent(payload)}`;
}

/**
 * The hall's address, with no slash at the end: the LAN address another device can reach,
 * or nothing when there is none. The demo has no LAN to enumerate, and the hall really is
 * this page's origin and the path the demo is published under, so there that is the honest
 * one to print.
 */
export function hallURL(clientURL: string): string {
  if (clientURL) return clientURL;
  return demo ? window.location.origin + import.meta.env.BASE_URL.replace(/\/+$/, '') : '';
}
