/**
 * The address a tablet should open, for the admin pages' join panels.
 *
 * It is emphatically NOT the admin page's own origin: the organizer's browser is on
 * http://localhost, which is the one address on this PC that no other device can reach.
 * The server enumerates the real ones and this picks between them -- the page's own when
 * it was itself opened over the network, then the one this PC chose last time, then the
 * server's best guess.
 *
 * The list follows the PC between networks without a reload. Joining the wrong wifi first
 * is a reasonable thing to have happen, and the organizer should see the right one appear
 * the moment the PC is on it. Only while the tab is visible: a background tab has nobody
 * looking at it.
 */
import { api, type Address } from '../api';

// Which network the QR code points at, remembered per PC. An organizer who had to pick
// once should not have to pick again after every restart.
const REMEMBERED = 'porta.clientAddress';

export class LanAddress {
  addresses = $state<Address[]>([]);
  chosenIP = $state('');
  private poll: ReturnType<typeof setInterval> | null = null;

  start(): void {
    void this.pick();
    this.poll = setInterval(() => {
      if (document.visibilityState === 'visible') void this.refresh();
    }, 5000);
  }

  stop(): void {
    if (this.poll) clearInterval(this.poll);
    this.poll = null;
  }

  get chosen(): Address | null {
    return this.addresses.find((a) => a.ip === this.chosenIP) ?? null;
  }

  /**
   * The base every client address is composed from. The port is this page's own: the
   * clients connect to the same server on the same port, so it never has to be
   * configured or passed through the API.
   */
  get clientURL(): string {
    const port = window.location.port ? `:${window.location.port}` : '';
    return this.chosenIP ? `http://${this.chosenIP}${port}` : '';
  }

  choose(ip: string): void {
    this.chosenIP = ip;
    try {
      localStorage.setItem(REMEMBERED, ip);
    } catch {
      // A browser with storage switched off still gets the choice, just not the memory.
    }
  }

  private async pick(): Promise<void> {
    try {
      this.addresses = await api.addresses();
    } catch {
      this.addresses = [];
    }
    this.choosePreferred();
  }

  /** Re-reads the list, and re-picks only if the chosen address has gone. */
  private async refresh(): Promise<void> {
    let next: Address[];
    try {
      next = await api.addresses();
    } catch {
      return;
    }
    if (JSON.stringify(next) === JSON.stringify(this.addresses)) return;
    this.addresses = next;
    if (!this.addresses.some((a) => a.ip === this.chosenIP)) this.choosePreferred();
  }

  private choosePreferred(): void {
    // If this page was itself opened over the network, that address is not a guess -- it
    // demonstrably works from at least one other device, which is more than the server's
    // ranking can know.
    const here = this.addresses.find((a) => a.ip === window.location.hostname);
    const remembered = this.addresses.find((a) => a.ip === remembering());
    this.chosenIP = (here ?? remembered ?? this.addresses[0])?.ip ?? '';
  }
}

// Guarded: a browser with site data switched off throws on access rather than returning
// null, and that must not take the join panel down with it.
function remembering(): string {
  try {
    return localStorage.getItem(REMEMBERED) ?? '';
  } catch {
    return '';
  }
}

/** "Wi-Fi Hall-Guest" when the network has a name; the adapter otherwise. */
export function describeNetwork(a: Address): string {
  return a.ssid ? `Wi-Fi ${a.ssid}` : a.interface;
}
