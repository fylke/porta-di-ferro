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
 */
import type { Event } from './match';

const DB_NAME = 'porta-di-ferro';
const DB_VERSION = 2;
const STORE = 'events';

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
      if (!db.objectStoreNames.contains(STORE)) {
        // Keyed on [match, seq] -- the same primary key the server uses, so writing the
        // same event twice is a no-op on both sides with no deduplication logic anywhere.
        const store = db.createObjectStore(STORE, { keyPath: ['match', 'seq'] });
        store.createIndex('byMatch', 'match', { unique: false });
      }
    };
    request.onsuccess = () => resolve(request.result);
    request.onerror = () => reject(request.error);
    request.onblocked = () => reject(new Error('indexedDB open blocked'));
  });
  return dbPromise;
}

interface StoredEvent extends Event {
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

/** Appends events to the durable log. Writing an existing (match, seq) is a no-op. */
export async function append(matchId: string, events: Event[]): Promise<void> {
  await attempt<void>(undefined, async (db) => {
    const tx = db.transaction(STORE, 'readwrite');
    const store = tx.objectStore(STORE);
    for (const e of events) {
      store.put({ ...e, match: matchId } satisfies StoredEvent);
    }
    await new Promise<void>((resolve, reject) => {
      tx.oncomplete = () => resolve();
      tx.onerror = () => reject(tx.error);
      tx.onabort = () => reject(tx.error);
    });
  });
}

/** Reads a match's durable log, in sequence order. Empty if storage is unavailable. */
export async function read(matchId: string): Promise<Event[]> {
  return attempt<Event[]>([], async (db) => {
    const index = db.transaction(STORE, 'readonly').objectStore(STORE).index('byMatch');
    const rows = await new Promise<StoredEvent[]>((resolve, reject) => {
      const req = index.getAll(IDBKeyRange.only(matchId));
      req.onsuccess = () => resolve(req.result as StoredEvent[]);
      req.onerror = () => reject(req.error);
    });
    return rows.sort((a, b) => a.seq - b.seq);
  });
}
