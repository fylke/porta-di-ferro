/**
 * Letting go of an event that has been deleted (#141).
 *
 * A device keeps what it needs to carry on alone: every match log it scored, in IndexedDB,
 * and a cached copy of the mats, the event and each discipline. All of it is keyed by
 * discipline and match -- "open-sabre/p1m1" -- and an event made after a delete, with the
 * same disciplines, has the same keys. A tablet that kept last month's p1m1 would push it
 * into this month's. So every event has an identity, and a device that sees a different
 * one from the one it last saw forgets what it kept of the old one and starts the page
 * again. What is about the device itself -- its id, language, mat side, folds -- stays.
 */
import * as db from './db';

const KEY = 'porta.eventId';

/** The keys that are about an event rather than about this device. */
const ABOUT_THE_EVENT = [
  /^porta\.mat\.\d+\.current$/,
  /^porta\.rest\./,
  /^porta\.mats$/,
  /^porta\.event$/,
  /^porta\.person\./,
  /^porta\.([^.]+\.)?snapshot$/,
];

export function aboutTheEvent(key: string): boolean {
  return ABOUT_THE_EVENT.some((re) => re.test(key));
}

/**
 * What seeing `id` means, after `known`. A device that has never been told an identity --
 * one from before identities, or a fresh one -- cannot tell, and keeps what it has.
 */
export function verdict(known: string, id: string | undefined): 'same' | 'first' | 'other' {
  if (!id || id === known) return 'same';
  return known ? 'other' : 'first';
}

let forgetting = false;

/**
 * Notes the event a view says it is from. True when it is another event than this device
 * last saw: what it kept is being forgotten and the page will start again, so the view
 * must not be used.
 */
export function anotherEvent(id: string | undefined): boolean {
  if (forgetting) return true;
  let known = '';
  try {
    known = localStorage.getItem(KEY) ?? '';
  } catch {
    // No storage: nothing kept to forget, beyond what the database may hold.
    return false;
  }
  const v = verdict(known, id);
  if (v === 'same') return false;
  if (v === 'first') {
    remember(id!);
    return false;
  }
  forgetting = true;
  void (async () => {
    await db.forgetAll();
    try {
      for (const key of Object.keys(localStorage)) if (aboutTheEvent(key)) localStorage.removeItem(key);
    } catch {
      // Fine without it: the database was what mattered.
    }
    // Remembered last, so a page closed half-way forgets again next time.
    remember(id!);
    window.location.reload();
  })();
  return true;
}

function remember(id: string) {
  try {
    localStorage.setItem(KEY, id);
  } catch {
    // Fine without it.
  }
}
