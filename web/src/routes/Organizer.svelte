<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '../api';
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
  let clientURL = $state('');

  onMount(() => {
    live.start();
    // The address a tablet should open is this page's own origin -- which is exactly what
    // the organizer's browser already knows, so there is nothing to configure.
    clientURL = window.location.origin;
    return () => live.stop();
  });

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
        <p class="url">{clientURL}</p>
        <p class="hint">
          Point a score keeper's device at that address, or let them scan the code. Every
          device on the venue wifi can reach it &mdash; including a spectator's own phone,
          for the roster and the mat scoreboards.
        </p>
        <p class="links">
          <a href="/score">Score keeper</a>
          <a href="/display/mat/1">Mat 1</a>
          {#if snapshot.tournament.mats > 1}<a href="/display/mat/2">Mat 2</a>{/if}
        </p>
      </div>
      {#if clientURL}
        <img class="qr" alt="QR code for {clientURL}" src="/api/qr.png?url={encodeURIComponent(clientURL)}" />
      {/if}
    </section>

    <div class="columns">
      <Competitors competitors={snapshot.competitors} poolsDrawn={drawn} onchange={refresh} />
      <Setup {snapshot} onchange={refresh} />
    </div>

    <Pools {snapshot} />
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
  .hint {
    margin: 0;
    color: var(--ink-dim);
    line-height: 1.55;
    max-width: 40rem;
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
