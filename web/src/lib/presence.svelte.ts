/**
 * This device's identity on the LAN, and its heartbeat to the server.
 *
 * A client announces itself with a stable id of its own choosing and then says hello
 * every few seconds. That is all the server needs to know who is alive: which device is
 * scoring which mat (so another can take over when it dies, and so the displays follow
 * the match it is holding), and which screens are waiting to be told what to show
 * (design §7 items 4 and 10).
 */
import { api, type Client } from '../api';

const ID_KEY = 'porta.clientId';

/** A stable id for this browser, made once and kept. */
export function clientId(): string {
  try {
    const have = localStorage.getItem(ID_KEY);
    if (have) return have;
    const fresh = crypto.randomUUID().replace(/-/g, '').slice(0, 16);
    localStorage.setItem(ID_KEY, fresh);
    return fresh;
  } catch {
    // No storage: a fresh id per load. Handover still works; the device just cannot be
    // recognised as the same one after a reload.
    return crypto.randomUUID().replace(/-/g, '').slice(0, 16);
  }
}

/**
 * What this device calls itself: "Tablet 7F3A". Short enough to read across a hall
 * and say out loud, distinct enough that two screens are two rows.
 */
export function deviceName(): string {
  const ua = navigator.userAgent;
  const kind = /iPad|Tablet/i.test(ua)
    ? 'Tablet'
    : /iPhone|Android.*Mobile/i.test(ua)
      ? 'Phone'
      : /Android/i.test(ua)
        ? 'Tablet'
        : 'Screen';
  return `${kind} ${clientId().slice(0, 4).toUpperCase()}`;
}

export interface HeartbeatFields {
  mat?: number;
  match?: string;
}

/**
 * Says hello every five seconds, and at once whenever what this device is doing changes.
 * The server's answer carries a display's assignment, which is the whole of
 * server-assigned displays from the device's side.
 */
export class Heartbeat {
  /** What the server last said about this device -- a display reads its target here. */
  me = $state<Client | null>(null);
  online = $state(true);

  private timer: ReturnType<typeof setInterval> | null = null;
  private fields: HeartbeatFields = {};
  private readonly role: 'scorekeeper' | 'display';
  private readonly id = clientId();

  constructor(role: 'scorekeeper' | 'display') {
    this.role = role;
  }

  start(fields: HeartbeatFields = {}): void {
    this.fields = fields;
    void this.beat();
    if (!this.timer) this.timer = setInterval(() => void this.beat(), 5000);
    // A closed tab is a graceful goodbye, as far as one can be sent from a page that is
    // going away; the server would notice on its own fifteen seconds later.
    window.addEventListener('pagehide', this.bye);
  }

  /** Something changed -- the match on the mat, say. Tell the server now, not in five seconds. */
  update(fields: HeartbeatFields): void {
    this.fields = { ...this.fields, ...fields };
    void this.beat();
  }

  private async beat(): Promise<void> {
    try {
      this.me = await api.register(this.id, { role: this.role, name: deviceName(), ...this.fields });
      this.online = true;
    } catch {
      this.online = false;
    }
  }

  private bye = (): void => {
    // Only a display says goodbye on its way out. A score keeper's release is a
    // deliberate act -- Hand over this mat -- because a tab that is merely reloading
    // must not let go of its match.
    if (this.role === 'display') api.releaseBeacon(this.id);
  };

  /** A deliberate goodbye: the device is done with what it was doing. */
  async release(): Promise<void> {
    this.stop();
    try {
      await api.release(this.id);
    } catch {
      // The server will notice on its own.
    }
  }

  stop(): void {
    if (this.timer) clearInterval(this.timer);
    this.timer = null;
    window.removeEventListener('pagehide', this.bye);
  }
}
