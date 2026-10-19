import { mp4, webm } from './nosleep-media';

/**
 * Keeps a screen awake. Both the score keeper's device and every display need it.
 *
 * The Wake Lock API is the way to ask, but browsers grant it only to a secure context, and
 * every device in the hall but the organizer's PC is on http://<LAN address> (issue #81).
 * There `navigator.wakeLock` is not there at all, and a tablet dimmed and locked itself in
 * the middle of a match (issue #144). So where it is missing or refused, this does what
 * NoSleep.js does: plays a second of silent video on a loop, out of sight, which every
 * mobile browser keeps the screen on for. A browser plays it only from a tap, so it starts
 * on the first one -- on a score keeper's tablet, the first point pressed -- and again on
 * the first tap after the page has been in the background, which pauses it.
 */
export function keepAwake(): () => void {
  let sentinel: WakeLockSentinel | null = null;
  let video: HTMLVideoElement | null = null;
  let released = false;

  const gestures = ['pointerdown', 'keydown', 'click'] as const;

  const playVideo = () => {
    if (released) return;
    if (!video) {
      video = document.createElement('video');
      video.setAttribute('title', 'Keep awake');
      video.setAttribute('playsinline', '');
      video.setAttribute('aria-hidden', 'true');
      // In the page and the size of it, because a browser may not count a video it cannot
      // see; but transparent, behind everything and never in the way of a tap.
      video.style.cssText =
        'position:fixed;inset:0;width:100%;height:100%;opacity:0.01;pointer-events:none;z-index:-1';
      for (const [type, src] of [
        ['video/webm', webm],
        ['video/mp4', mp4],
      ]) {
        const source = document.createElement('source');
        source.type = type;
        source.src = src;
        video.appendChild(source);
      }
      const v = video;
      v.addEventListener('loadedmetadata', () => {
        // The WebM is shorter than a second and loops; the MP4 is longer and is kept in
        // its first half, as NoSleep.js does.
        if (v.duration <= 1) v.loop = true;
        else
          v.addEventListener('timeupdate', () => {
            if (v.currentTime > 0.5) v.currentTime = Math.random();
          });
      });
      document.body.appendChild(video);
    }
    if (!video.paused) return;
    video.play().then(
      () => disarm(),
      () => {
        // Not from a tap, or not allowed: the next tap tries again.
      },
    );
  };

  const arm = () => {
    for (const g of gestures) document.addEventListener(g, playVideo, { capture: true, passive: true });
  };
  const disarm = () => {
    for (const g of gestures) document.removeEventListener(g, playVideo, { capture: true });
  };

  const request = async () => {
    if (released) return;
    try {
      if (!navigator.wakeLock) throw new Error('no wake lock here');
      sentinel = await navigator.wakeLock.request('screen');
      sentinel.addEventListener('release', () => {
        sentinel = null;
      });
    } catch {
      // Unsupported, or refused: the video instead. Tried now, which a browser that lets
      // a page play on its own allows, and otherwise on the next tap.
      playVideo();
      arm();
    }
  };

  const onVisible = () => {
    if (document.visibilityState !== 'visible') return;
    if (video) {
      if (video.paused) {
        playVideo();
        arm();
      }
    } else if (!sentinel) void request();
  };

  void request();
  document.addEventListener('visibilitychange', onVisible);

  return () => {
    released = true;
    disarm();
    document.removeEventListener('visibilitychange', onVisible);
    void sentinel?.release();
    sentinel = null;
    if (video) {
      video.pause();
      video.remove();
      video = null;
    }
  };
}
