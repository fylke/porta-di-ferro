/**
 * IndexedDB: the score keeper client's own append-only log.
 *
 * A tap writes here and returns. Pushing to the server is asynchronous, so the score
 * keeper never waits on the LAN and a lost device costs at most one exchange
 * (design §3).
 *
 * This is also what makes tier 2 of the fallback ladder free: a client that never reaches
 * the server at all still runs a full match, because it holds the whole log itself.
 */
import type { Event } from './match';

const DB_NAME = 'porta-di-ferro';
const DB_VERSION = 1;
const STORE = 'events';

let dbPromise: Promise<IDBDatabase> | null = null;

function open(): Promise<IDBDatabase> {
  if (dbPromise) return dbPromise;
  dbPromise = new Promise((resolve, reject) => {
    const request = indexedDB.open(DB_NAME, DB_VERSION);
    request.onupgradeneeded = () => {
      const db = request.result;
      if (!db.objectStoreNames.contains(STORE)) {
        // Keyed on [match, seq] -- the same primary key the server uses, so writing the
        // same event twice is a no-op on both sides with no deduplication logic anywhere.
        const store = db.createObjectStore(STORE, { keyPath: ['match', 'seq'] });
        store.createIndex('byMatch', 'match', { unique: false });
        store.createIndex('unpushed', ['match', 'pushed'], { unique: false });
      }
    };
    request.onsuccess = () => resolve(request.result);
    request.onerror = () => reject(request.error);
  });
  return dbPromise;
}

interface StoredEvent extends Event {
  match: string;
  pushed: number;
}

function tx(db: IDBDatabase, mode: IDBTransactionMode): IDBObjectStore {
  return db.transaction(STORE, mode).objectStore(STORE);
}

function wrap<T>(request: IDBRequest<T>): Promise<T> {
  return new Promise((resolve, reject) => {
    request.onsuccess = () => resolve(request.result);
    request.onerror = () => reject(request.error);
  });
}

/** Appends events to the local log. Writing an existing (match, seq) is a no-op. */
export async function append(matchId: string, events: Event[]): Promise<void> {
  const db = await open();
  const store = tx(db, 'readwrite');
  for (const e of events) {
    const stored: StoredEvent = { ...e, match: matchId, pushed: 0 };
    store.put(stored);
  }
  await new Promise<void>((resolve, reject) => {
    store.transaction.oncomplete = () => resolve();
    store.transaction.onerror = () => reject(store.transaction.error);
  });
}

/** Reads a match's whole local log, in sequence order. */
export async function read(matchId: string): Promise<Event[]> {
  const db = await open();
  const index = tx(db, 'readonly').index('byMatch');
  const rows = await wrap(index.getAll(IDBKeyRange.only(matchId)));
  return (rows as StoredEvent[])
    .sort((a, b) => a.seq - b.seq)
    .map(({ pushed: _pushed, ...event }) => event as Event);
}

/** The events this device has written but not yet handed to the server. */
export async function unpushed(matchId: string): Promise<Event[]> {
  const db = await open();
  const index = tx(db, 'readonly').index('unpushed');
  const rows = await wrap(index.getAll(IDBKeyRange.only([matchId, 0])));
  return (rows as StoredEvent[])
    .sort((a, b) => a.seq - b.seq)
    .map(({ pushed: _pushed, ...event }) => event as Event);
}

/** Marks events as accepted by the server. */
export async function markPushed(matchId: string, seqs: number[]): Promise<void> {
  const db = await open();
  const store = tx(db, 'readwrite');
  for (const seq of seqs) {
    const request = store.get([matchId, seq]);
    request.onsuccess = () => {
      const row = request.result as StoredEvent | undefined;
      if (row) store.put({ ...row, pushed: 1 });
    };
  }
  await new Promise<void>((resolve, reject) => {
    store.transaction.oncomplete = () => resolve();
    store.transaction.onerror = () => reject(store.transaction.error);
  });
}

/** True when this browser can hold a log at all. Safari private mode cannot. */
export function available(): boolean {
  try {
    return typeof indexedDB !== 'undefined';
  } catch {
    return false;
  }
}
