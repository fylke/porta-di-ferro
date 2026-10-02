/**
 * IndexedDB: the score keeper client's durable copy of the match log.
 *
 * A tap writes here and returns. Pushing to the server is asynchronous, so the score
 * keeper never waits on the LAN and a lost device costs at most one exchange (design §3).
 *
 * **Every function here is best-effort and never throws.** `indexedDB` being defined is
 * not the same as it working: Safari in private browsing, a storage quota, or a browser
 * with site data blocked all fail at `open()` or later, and some throw on the accessor
 * itself. Losing durability means a refresh loses the match; letting that failure reach
 * the caller would mean the score keeper cannot score at all, which is far worse. So the
 * failure is reported through `usable()` and the match runs from memory.
 *
 * **Every row says which discipline it belongs to.** An event's disciplines share one
 * address, so they share this database, and "p1m1" exists in every one of them. The key
 * is [discipline, match, seq], so two disciplines' logs never meet and an unsent exchange
 * is always sent to the discipline it was scored in, whatever page is open when the LAN
 * comes back (docs/proposals/one-event-many-disciplines.md §12). A row from before events
 * existed has the discipline '' -- the unprefixed paths, which answer as the event's one
 * discipline, the only kind of event that device can have been scoring for.
 */
import type { Event } from './match';

const DB_NAME = 'porta-di-ferro';
const DB_VERSION = 3;
const STORE = 'log';
/** The store before disciplines, keyed [match, seq]. Moved into STORE on upgrade. */
const OLD_STORE = 'events';

let dbPromise: Promise<IDBDatabase> | null = null;
let broken = false;

/** False once a storage operation has failed. The log then lives only in memory. */
export function usable(): boolean {
  return !broken;
}

function open(): Promise<IDBDatabase> {
  if (dbPromise) return dbPromise;
  dbPromise = new Promise((resolve, reject) => {
    let request: IDBOpenDBRequest;
    try {
      request = indexedDB.open(DB_NAME, DB_VERSION);
    } catch (e) {
      // Some browsers throw on the accessor rather than rejecting the request.
      reject(e);
      return;
    }
    request.onupgradeneeded = () => {
      const db = request.result;
      const tx = request.transaction!;
      if (!db.objectStoreNames.contains(STORE)) {
        // The same primary key the server uses, plus the discipline: writing the same
        // event twice is a no-op on both sides with no deduplication logic anywhere.
        const store = db.createObjectStore(STORE, { keyPath: ['discipline', 'match', 'seq'] });
        store.createIndex('byMatch', ['discipline', 'match'], { unique: false });
      }
      if (db.objectStoreNames.contains(OLD_STORE)) {
        // Unsent exchanges from before the upgrade must survive it: they are what the
        // durable log exists for.
        const from = tx.objectStore(OLD_STORE);
        const to = tx.objectStore(STORE);
        from.openCursor().onsuccess = (ev) => {
          const cursor = (ev.target as IDBRequest<IDBCursorWithValue | null>).result;
          if (!cursor) {
            db.deleteObjectStore(OLD_STORE);
            return;
          }
          to.put({ ...cursor.value, discipline: '' });
          cursor.continue();
        };
      }
    };
    request.onsuccess = () => resolve(request.result);
    request.onerror = () => reject(request.error);
    request.onblocked = () => reject(new Error('indexedDB open blocked'));
  });
  return dbPromise;
}

interface StoredEvent extends Event {
  discipline: string;
  match: string;
}

/** A match this device holds a log for, and the discipline it was scored in. */
export interface HeldMatch {
  discipline: string;
  match: string;
}

/** Runs a storage operation, and gives up on storage for good if it fails. */
async function attempt<T>(fallback: T, fn: (db: IDBDatabase) => Promise<T>): Promise<T> {
  if (broken) return fallback;
  try {
    return await fn(await open());
  } catch (e) {
    broken = true;
    console.warn('porta: local storage unavailable, running from memory', e);
    return fallback;
  }
}

function done(tx: IDBTransaction): Promise<void> {
  return new Promise<void>((resolve, reject) => {
    tx.oncomplete = () => resolve();
    tx.onerror = () => reject(tx.error);
    tx.onabort = () => reject(tx.error);
  });
}

/** Appends events to the durable log. Writing an existing (discipline, match, seq) is a no-op. */
export async function append(discipline: string, matchId: string, events: Event[]): Promise<void> {
  await attempt<void>(undefined, async (db) => {
    const tx = db.transaction(STORE, 'readwrite');
    const store = tx.objectStore(STORE);
    for (const e of events) {
      store.put({ ...e, discipline, match: matchId } satisfies StoredEvent);
    }
    await done(tx);
  });
}

/** Drops a match's durable log, for when the server's copy has been rewritten over it. */
export async function clear(discipline: string, matchId: string): Promise<void> {
  await attempt<void>(undefined, async (db) => {
    const tx = db.transaction(STORE, 'readwrite');
    const index = tx.objectStore(STORE).index('byMatch');
    const keys = await new Promise<IDBValidKey[]>((resolve, reject) => {
      const req = index.getAllKeys(IDBKeyRange.only([discipline, matchId]));
      req.onsuccess = () => resolve(req.result);
      req.onerror = () => reject(req.error);
    });
    // The index yields primary keys -- [discipline, match, seq] -- which is what delete wants.
    for (const key of keys) tx.objectStore(STORE).delete(key);
    await done(tx);
  });
}

/** Every match this device holds a log for. Empty if storage is unavailable. */
export async function matches(): Promise<HeldMatch[]> {
  return attempt<HeldMatch[]>([], async (db) => {
    const store = db.transaction(STORE, 'readonly').objectStore(STORE);
    const keys = await new Promise<IDBValidKey[]>((resolve, reject) => {
      const req = store.getAllKeys();
      req.onsuccess = () => resolve(req.result);
      req.onerror = () => reject(req.error);
    });
    const seen = new Map<string, HeldMatch>();
    for (const key of keys) {
      const [discipline, match] = key as [string, string, number];
      seen.set(`${discipline}\u0000${match}`, { discipline, match });
    }
    return [...seen.values()];
  });
}

/** Reads a match's durable log, in sequence order. Empty if storage is unavailable. */
export async function read(discipline: string, matchId: string): Promise<Event[]> {
  return attempt<Event[]>([], async (db) => {
    const index = db.transaction(STORE, 'readonly').objectStore(STORE).index('byMatch');
    const rows = await new Promise<StoredEvent[]>((resolve, reject) => {
      const req = index.getAll(IDBKeyRange.only([discipline, matchId]));
      req.onsuccess = () => resolve(req.result as StoredEvent[]);
      req.onerror = () => reject(req.error);
    });
    // The discipline is where a row is kept, not part of the event the server is sent.
    return rows.map(({ discipline: _, ...e }) => e as Event).sort((a, b) => a.seq - b.seq);
  });
}
