<script lang="ts">
  import { onMount } from 'svelte';
  import { api, type Address } from '../api';
  import { Live } from '../lib/live.svelte';
  import Competitors from './Competitors.svelte';
  import Setup from './Setup.svelte';
  import Pools from './Pools.svelte';

  /**
   * The first screen an organizer sees. It carries the LAN address and a QR code large
   * enough to read from across a table, because "now open your browser" is the cost of
   * choosing a server over a desktop application and this is the mitigation
   * (docs/tech-stack.md §2).
   */
  const live = new Live();
  let addresses = $state<Address[]>([]);
  let chosenIP = $state('');

  // Which network the QR code points at, remembered per PC. An organizer who had to pick
  // once should not have to pick again after every restart.
  const REMEMBERED = 'porta.clientAddress';

  onMount(() => {
    live.start();
    void pickAddress();
    return () => live.stop();
  });

  /**
   * The address a tablet should open. It is emphatically NOT this page's own origin: the
   * organizer's browser is on http://localhost, which is the one address on this PC that
   * no other device can reach. The server enumerates the real ones and this picks between
   * them.
   */
  async function pickAddress() {
    try {
      addresses = await api.addresses();
    } catch {
      addresses = [];
    }
    // If this page was itself opened over the network, that address is not a guess -- it
    // demonstrably works from at least one other device, which is more than the server's
    // ranking can know.
    const here = addresses.find((a) => a.ip === window.location.hostname);
    const remembered = addresses.find((a) => a.ip === remembering());
    chosenIP = (here ?? remembered ?? addresses[0])?.ip ?? '';
  }

  // Both sides of the memory are guarded: a browser with site data switched off throws on
  // access rather than returning null, and that must not take the join panel down with it.
  function remembering(): string {
    try {
      return localStorage.getItem(REMEMBERED) ?? '';
    } catch {
      return '';
    }
  }

  function choose(ip: string) {
    chosenIP = ip;
    try {
      localStorage.setItem(REMEMBERED, ip);
    } catch {
      // A browser with storage switched off still gets the choice, just not the memory.
    }
  }

  // The port is this page's own: the clients connect to the same server on the same port,
  // so it never has to be configured or passed through the API.
  const port = $derived(window.location.port ? `:${window.location.port}` : '');
  const clientURL = $derived(chosenIP ? `http://${chosenIP}${port}` : '');
  /**
   * What the QR code encodes, and what an organizer reads out: the score keeper's own
   * page, not the front door.
   *
   * Scanning the code used to land a tablet on this very screen -- the competitor
   * register, the setup, the pool tables -- with the mat picker somewhere below it. That
   * is the organizer's page on the organizer's PC, and none of it is any use at a mat.
   */
  const scoreURL = $derived(clientURL ? `${clientURL}/score` : '');

  async function refresh() {
    try {
      live.snapshot = await api.state();
    } catch {
      // The stream will bring it along shortly.
    }
  }

  const snapshot = $derived(live.snapshot);
  const drawn = $derived((snapshot?.pools.length ?? 0) > 0);
</script>

<main>
  <header>
    <h1>Porta di Ferro</h1>
    <nav>
      <a href="/display/mats" target="_blank" rel="noreferrer">Displays</a>
      <a href="/display/roster" target="_blank" rel="noreferrer">Roster</a>
      <a href="/print/pools" target="_blank" rel="noreferrer">Pool sheets</a>
      <a href="/api/export.json">Export JSON</a>
    </nav>
  </header>

  {#if !snapshot}
    <p class="loading">{live.error || 'Loading…'}</p>
  {:else}
    <section class="join">
      <div>
        <h2>Join from a tablet or phone</h2>
        {#if clientURL}
          <p class="url">{scoreURL}</p>
          {#if addresses.length > 1}
            <label class="network">
              Network
              <select value={chosenIP} onchange={(e) => choose(e.currentTarget.value)}>
                {#each addresses as a (a.ip)}
                  <option value={a.ip}>{a.interface} &mdash; {a.ip}</option>
                {/each}
              </select>
            </label>
            <p class="hint">
              This PC is on more than one network. Pick the one the tablets are on; the
              address and the code above follow the choice, and it is remembered.
            </p>
          {:else}
            <p class="hint">
              Point a score keeper's device at that address, or let them scan the code. It
              opens straight on the mat picker.
            </p>
          {/if}
        {:else}
          <p class="url none">No network</p>
          <p class="hint warn">
            This PC is not on a network another device could reach, so there is no address
            to hand out. Join it to the venue wifi and reload this page. Scoring on this PC
            still works.
          </p>
        {/if}
        {#if clientURL}
          <p class="hint">
            Spare screens and spectators&rsquo; phones open
            <span class="mono">{clientURL}/display/mats</span> or
            <span class="mono">{clientURL}/display/roster</span>. Any device on the venue
            wifi can reach them.
          </p>
        {/if}
        <p class="links">
          <a href="/score">Score keeper</a>
          {#each { length: snapshot.tournament.mats } as _, i (i)}
            <a href="/display/mat/{i + 1}">Mat {i + 1}</a>
          {/each}
        </p>
      </div>
      {#if scoreURL}
        <img class="qr" alt="QR code for {scoreURL}" src="/api/qr.png?url={encodeURIComponent(scoreURL)}" />
      {/if}
    </section>

    <div class="columns">
      <Competitors competitors={snapshot.competitors} poolsDrawn={drawn} onchange={refresh} />
      <Setup {snapshot} onchange={refresh} />
    </div>

    <Pools {snapshot} onchange={refresh} />
  {/if}
</main>

<style>
  main {
    max-width: 68rem;
    margin: 0 auto;
    padding: 1.25rem 1.25rem 4rem;
  }
  header {
    display: flex;
    flex-wrap: wrap;
    gap: 1rem;
    justify-content: space-between;
    align-items: baseline;
    margin-bottom: 1.25rem;
  }
  h1 {
    margin: 0;
    font-size: 1.5rem;
  }
  nav {
    display: flex;
    gap: 1rem;
    font-size: 0.9rem;
  }
  .join {
    display: grid;
    grid-template-columns: 1fr auto;
    gap: 1.5rem;
    align-items: center;
    background: var(--panel);
    border-radius: var(--radius);
    padding: 1.1rem 1.25rem;
    margin-bottom: 1rem;
  }
  @media (max-width: 640px) {
    .join {
      grid-template-columns: 1fr;
    }
  }
  h2 {
    margin: 0 0 0.5rem;
    font-size: 1.15rem;
  }
  .url {
    margin: 0 0 0.5rem;
    font-size: clamp(1.2rem, 3vw, 1.8rem);
    font-weight: 700;
    font-variant-numeric: tabular-nums;
  }
  .network {
    display: inline-flex;
    align-items: center;
    gap: 0.6rem;
    margin-bottom: 0.6rem;
    font-size: 0.9rem;
    color: var(--ink-dim);
  }
  .url.none {
    color: var(--ink-dim);
  }
  .hint.warn {
    color: var(--amber-bright);
  }
  .hint {
    margin: 0 0 0.4rem;
    color: var(--ink-dim);
    line-height: 1.55;
    max-width: 40rem;
  }
  .hint .mono {
    color: var(--ink);
  }
  .links {
    display: flex;
    gap: 1rem;
    margin: 0.75rem 0 0;
    font-size: 0.9rem;
  }
  .qr {
    width: clamp(9rem, 22vw, 13rem);
    height: auto;
    background: #fff;
    padding: 0.5rem;
    border-radius: var(--radius);
  }
  .columns {
    display: grid;
    grid-template-columns: 1.3fr 1fr;
    gap: 1rem;
    margin-bottom: 1rem;
    align-items: start;
  }
  @media (max-width: 860px) {
    .columns {
      grid-template-columns: 1fr;
    }
  }
  .loading {
    color: var(--ink-dim);
  }
</style>
