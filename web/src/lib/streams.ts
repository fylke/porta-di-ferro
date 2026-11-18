/**
 * One stream per page (#161).
 *
 * A browser allows six HTTP/1.1 connections to one address, and a page used to hold up to
 * three Server-Sent Events streams of its own -- the event's, the mats' and one for each
 * discipline it shows. Three organizer tabs used every connection between them, and then no
 * tab of that browser could make a request at all: dead buttons, with nothing to say why.
 *
 * So everything on a page that follows the server follows it here, over one connection to
 * /api/streams with a topic for each, and the connection is let go while the tab is hidden.
 * Coming back, it opens again, and every follower's onOpen asks for a fresh picture, as each
 * already did when its own stream reopened. A display is never hidden, so it never notices.
 */

/** One update, with the stream it came from. */
export interface Tagged {
  topic: string;
  kind: string;
  match?: string;
  data?: unknown;
}

export interface Follower {
  onMessage: (update: Tagged) => void;
  /** The stream is open, after a gap or for the first time: ask for the whole picture. */
  onOpen?: () => void;
  /** The stream is down; the browser, or this module, will open it again. */
  onClose?: () => void;
}

/** How long a hidden tab keeps its stream: long enough for a quick look at another tab. */
const HIDDEN_GRACE_MS = 15_000;

const followers = new Map<string, Set<Follower>>();
let source: EventSource | null = null;
let openFor = '';
let pending = false;
let hidden = false;
let hiding: ReturnType<typeof setTimeout> | null = null;

/** Follows a topic -- "event", "mats" or "d/{slug}" -- until the returned function is called. */
export function follow(topic: string, f: Follower): () => void {
  let set = followers.get(topic);
  if (!set) followers.set(topic, (set = new Set()));
  set.add(f);
  schedule();
  // Joining a stream that is already open: it will not open again for this follower.
  if (source && source.readyState === 1 && openFor.split('&').includes(topicParam(topic))) {
    queueMicrotask(() => f.onOpen?.());
  }
  return () => {
    followers.get(topic)?.delete(f);
    schedule();
  };
}

function topicParam(topic: string): string {
  return `topic=${encodeURIComponent(topic)}`;
}

/** Topics come and go together as a page mounts: the connection is reworked once. */
function schedule(): void {
  if (pending) return;
  pending = true;
  queueMicrotask(() => {
    pending = false;
    reconcile();
  });
}

function each(fn: (f: Follower) => void): void {
  for (const set of followers.values()) for (const f of set) fn(f);
}

function close(): void {
  if (!source) return;
  source.close();
  source = null;
  openFor = '';
  each((f) => f.onClose?.());
}

function reconcile(): void {
  const topics = [...followers.entries()]
    .filter(([, set]) => set.size > 0)
    .map(([t]) => t)
    .sort();
  const query = topics.map(topicParam).join('&');
  if (hidden || topics.length === 0) {
    close();
    return;
  }
  if (source && query === openFor) return;
  close();
  const s = new EventSource(`/api/streams?${query}`);
  source = s;
  openFor = query;
  s.onopen = () => each((f) => f.onOpen?.());
  s.onmessage = (ev) => {
    let update: Tagged;
    try {
      update = JSON.parse(ev.data) as Tagged;
    } catch {
      // A malformed frame is not worth taking the stream down for.
      return;
    }
    for (const f of followers.get(update.topic) ?? []) f.onMessage(update);
  };
  s.onerror = () => {
    // EventSource retries on its own; the followers only say so on screen.
    each((f) => f.onClose?.());
  };
}

if (typeof document !== 'undefined') {
  document.addEventListener('visibilitychange', () => {
    if (document.visibilityState === 'hidden') {
      if (hiding) return;
      hiding = setTimeout(() => {
        hiding = null;
        hidden = true;
        reconcile();
      }, HIDDEN_GRACE_MS);
      return;
    }
    if (hiding) clearTimeout(hiding);
    hiding = null;
    if (hidden) {
      hidden = false;
      reconcile();
    }
  });
}
