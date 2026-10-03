<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '../api';
  import { Live } from '../lib/live.svelte';
  import { LanAddress, describeNetwork } from '../lib/lan.svelte';
  import { hallURL, qrSrc } from '../lib/qr';
  import { apiBase } from '../lib/paths';
  import { dhref, discipline } from '../router.svelte';
  import Competitors from './Competitors.svelte';
  import Setup from './Setup.svelte';
  import Pools from './Pools.svelte';
  import Eliminations from './Eliminations.svelte';
  import Screens from './Screens.svelte';
  import Disciplines from './Disciplines.svelte';
  import MatBoard from './MatBoard.svelte';
  import Planning from './Planning.svelte';
  import Staff from './Staff.svelte';
  import EventEditor from './EventEditor.svelte';
  import Signup from './Signup.svelte';
  import LangToggle from './LangToggle.svelte';
  import { t, lang } from '../lib/i18n.svelte';

  /**
   * One discipline's admin view (issue #98): everything that changes the tournament, and
   * nothing that does not. At /admin while the event has one discipline, and at
   * /d/{discipline}/admin -- or /admin/{discipline} -- once it has several, when /admin
   * itself is the event's page (docs/proposals/one-event-many-disciplines.md §6).
   *
   * It used to be the landing page. It is not any more, because the address on the poster
   * by the door reaches every phone in the hall and this page can withdraw a competitor
   * and redraw the pools.
   *
   * It carries the LAN address and a QR code large enough to read from across a table,
   * because "now open your browser" is the cost of choosing a server over a desktop
   * application and this is the mitigation (docs/tech-stack.md §2).
   */
  let { multi = false }: { multi?: boolean } = $props();

  const live = new Live();
  const lan = new LanAddress();
  // Substituted at build time, so this is a constant the normal build evaluates to
  // false and Rollup removes along with the branch it guards.
  const demo = import.meta.env.VITE_DEMO === 'true';
  const base = apiBase(discipline());

  onMount(() => {
    live.start();
    lan.start();
    // Who is connected arrives over the stream from then on; this is the first copy.
    api
      .presence()
      .then((p) => (live.presence = p))
      .catch(() => {
        // The stream brings it along.
      });
    return () => {
      lan.stop();
      live.stop();
    };
  });

  const clientURL = $derived(lan.clientURL);
  /**
   * What the QR code encodes, and what an organizer reads out: the score keeper's own
   * page, not the front door.
   *
   * Scanning the code used to land a tablet on this very screen -- the competitor
   * register, the setup, the pool tables -- with the mat picker somewhere below it. That
   * is the organizer's page on the organizer's PC, and none of it is any use at a mat.
   */
  // The score keepers' page is the hall's: a tablet picks a physical mat (phase 2).
  const scoreURL = $derived(hallURL(clientURL) ? `${hallURL(clientURL)}/score` : '');

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
    <h1>
      Porta di Ferro
      <span class="admintag">{t('admin')}</span>
      {#if snapshot?.instance.name}<span class="discipline">{snapshot.instance.name}</span>{/if}
    </h1>
    <nav>
      {#if multi}<a href="/admin">&larr; {t('The event')}</a>{/if}
      <a href={dhref('/')}>{t('Landing page')}</a>
      <a href="/info">{t('Info sheet')}</a>
      <a href="/display/mats" target="_blank" rel="noreferrer">{t('Displays')}</a>
      <a href={dhref('/display/roster')} target="_blank" rel="noreferrer">{t('Roster')}</a>
      <a href={dhref('/print/pools')} target="_blank" rel="noreferrer">{t('Pool sheets')}</a>
      <a href="{base}/export.json">{t('Export JSON')}</a>
      <a href="{base}/export.pdf?lang={lang.current}">{t('Export PDF')}</a>
      <LangToggle />
    </nav>
  </header>

  {#if !snapshot}
    <p class="loading">{live.error || t('Loading…')}</p>
  {:else}
    <section class="join">
      <div>
        <h2>{t('Join from a tablet or phone')}</h2>
        {#if clientURL}
          <p class="url">{scoreURL}</p>
          {#if lan.chosen}
            <p class="on">{t('on {network}', { network: describeNetwork(lan.chosen) })}</p>
          {/if}
          {#if lan.addresses.length > 1}
            <label class="network">
              {t('Network')}
              <select value={lan.chosenIP} onchange={(e) => lan.choose(e.currentTarget.value)}>
                {#each lan.addresses as a (a.ip)}
                  <option value={a.ip}>{describeNetwork(a)}: {a.ip}</option>
                {/each}
              </select>
            </label>
            <p class="hint">{t('This PC is on more than one network. Pick the one the tablets are on; the address and the code above follow the choice, and it is remembered.')}</p>
          {:else}
            <p class="hint">{t("Point a score keeper's device at that address, or let them scan the code. It opens straight on the mat picker.")}</p>
          {/if}
        {:else if demo}
          <!-- The one place the application is told it is a demo (issue #88).
               A browser tab is on no LAN, which is true and which the panel below would
               otherwise report as an orange warning about the venue wifi -- the first
               thing a visitor sees, about a problem they do not have. What is true here
               is that every client opens in this same tab, so it says that instead, and
               keeps the links, which are the part worth clicking. -->
          <p class="url none">{t('One tab, every screen')}</p>
          <p class="hint">{t('At an event these open on the score keepers\' tablets and the hall screens, over the venue wifi. Here they open in this tab, off the same tournament. Try one:')}</p>
        {:else}
          <p class="url none">{t('No network')}</p>
          <p class="hint warn">{t('This PC is not on a network another device could reach, so there is no address to hand out. Join it to the venue wifi and reload this page. Scoring on this PC still works.')}</p>
        {/if}
        {#if clientURL}
          <p class="hint">
            {t('Spare screens open')} <span class="mono">{clientURL}/display</span>
            {t('and are told what to show from here, under Screens — or go straight to')}
            <span class="mono">{clientURL}/display/mats</span> {t('or')}
            <span class="mono">{clientURL}{dhref('/display/roster')}</span>. {t('Any device on the venue wifi can reach them.')}
          </p>
        {/if}
        <p class="links">
          <a href="/score">{t('Score keeper')}</a>
          <a href="/print/mats" target="_blank" rel="noreferrer">{t('Print a code for each mat')}</a>
          {#each { length: snapshot.eventMats ?? snapshot.tournament.mats } as _, i (i)}
            <a href="/display/mat/{i + 1}">{t('Mat {n}', { n: i + 1 })}</a>
          {/each}
        </p>
      </div>
      {#if scoreURL}
        <img class="qr" alt={t('QR code for {url}', { url: scoreURL })} src={qrSrc(scoreURL)} />
      {/if}
    </section>

    <div class="columns">
      <Competitors competitors={snapshot.competitors} poolsDrawn={drawn} onchange={refresh} {multi} />
      <Setup {snapshot} onchange={refresh} />
    </div>

    <!-- The day around the fencing belongs to the event. With one discipline it is
         edited here, as it always was; with several, once, on the event's own page. -->
    {#if !multi}
      <EventEditor event={snapshot.tournament.event ?? {}} save={api.saveEvent} onchange={refresh} />
    {/if}

    <Signup {snapshot} onchange={refresh} scope={multi ? 'share' : 'single'} />

    <!-- Screens and score keepers are the event's: here while there is one discipline,
         on the event's page once there are several. -->
    {#if !multi}<Screens />{/if}

    <Eliminations {snapshot} onchange={refresh} />

    {#if !multi}
      <Disciplines onchange={refresh} />
    {/if}

    <!-- The mat board, for dragging pools between mats (#101). With several disciplines it
         is on the event's page, where it shows all of them. -->
    <!-- Before the draw the board shows the day as it would be (phase 4). -->
    {#if !multi && (drawn || snapshot.competitors.length > 0)}<MatBoard />{/if}
    {#if !multi}<Planning />{/if}
    {#if !multi}<Staff />{/if}

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
    display: flex;
    align-items: baseline;
    gap: 0.6rem;
  }
  /* Which discipline this tab is. Amber, so two tabs of two disciplines cannot be told
     apart only by reading the small print. */
  /* Says which of the three views this is, since they share a header. */
  .admintag {
    font-size: 0.68rem;
    font-weight: 800;
    letter-spacing: 0.12em;
    text-transform: uppercase;
    color: #0d0f14;
    background: var(--amber-bright);
    padding: 0.1rem 0.4rem;
    border-radius: 4px;
    vertical-align: 0.2em;
  }
  .discipline {
    font-size: 1.1rem;
    font-weight: 600;
    color: var(--amber-bright);
  }
  nav {
    display: flex;
    flex-wrap: wrap;
    gap: 0.5rem 1rem;
    font-size: 0.9rem;
  }
  .join {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
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
  .on {
    margin: -0.3rem 0 0.6rem;
    color: var(--ink-dim);
    font-size: 0.95rem;
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
    grid-template-columns: minmax(0, 1.3fr) minmax(0, 1fr);
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
