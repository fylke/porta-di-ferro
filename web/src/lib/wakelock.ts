/**
 * Keeps a screen awake. Both the score keeper's device and every display need it, and it
 * is Safari 16.4+ only -- so this asks and carries on rather than depending on it.
 */
export function keepAwake(): () => void {
  let sentinel: WakeLockSentinel | null = null;
  let released = false;

  const request = async () => {
    if (released) return;
    try {
      sentinel = await navigator.wakeLock?.request('screen');
      sentinel?.addEventListener('release', () => {
        sentinel = null;
      });
    } catch {
      // Unsupported, or refused because the page is hidden. Not worth telling anyone
      // about: the fallback is that the device sleeps as it normally would.
    }
  };

  const onVisible = () => {
    if (document.visibilityState === 'visible' && !sentinel) void request();
  };

  void request();
  document.addEventListener('visibilitychange', onVisible);

  return () => {
    released = true;
    document.removeEventListener('visibilitychange', onVisible);
    void sentinel?.release();
    sentinel = null;
  };
}
