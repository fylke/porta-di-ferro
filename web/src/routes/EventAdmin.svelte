<script lang="ts">
  import { onMount } from 'svelte';
  import { hall } from '../lib/event.svelte';
  import { navigate, query } from '../router.svelte';
  import { MatsLive } from '../lib/mats.svelte';
  import { LanAddress, describeNetwork } from '../lib/lan.svelte';
  import { hallURL, qrSrc } from '../lib/qr';
  import Disciplines from './Disciplines.svelte';
  import MatBoard from './MatBoard.svelte';
  import Screens from './Screens.svelte';
  import People from './People.svelte';
  import Staff from './Staff.svelte';
  import Signup from './Signup.svelte';
  import LangToggle from './LangToggle.svelte';
  import DeleteEvent from './DeleteEvent.svelte';
  import { t } from '../lib/i18n.svelte';

  /**
   * The event's admin view, at /admin once the event runs more than one discipline
   * (docs/proposals/one-event-many-disciplines.md §6): what is about the event rather than
   * any one discipline. Each discipline's competitors, pools and matches are on its own
   * admin page, one tap away; what is set once before the day is in Plan the event.
   *
   * Two dashboards under one header (#142), because what an organizer needs before the
   * doors open is not what they watch all day: before, the signup, the people it makes and
   * the staff; on the day, the mats and the screens. The page opens on the one that fits --
   * on the day once a match anywhere has been fenced -- and the other is a tab away, in the
   * address (?view=before), for the late entry or the volunteer who did not turn up. The
   * join panels, the hall's address and the score keepers', are on both.
   */
  const lan = new LanAddress();
  // The hall's mats, for a link to each one's display (#140).
  const mats = new MatsLive();
  const demo = import.meta.env.VITE_DEMO === 'true';

  onMount(() => {
    void hall.refresh();
    hall.follow();
    lan.start();
    void mats.start();
    return () => {
      mats.stop();
      lan.stop();
      hall.unfollow();
    };
  });

  const view = $derived(hall.view);
  const started = $derived(
    (view?.disciplines ?? []).some((d) => d.matchesDone > 0 || d.stage === 'eliminations' || d.stage === 'done'),
  );
  const chosen = $derived(query().get('view'));
  const dashboard = $derived(chosen === 'before' || chosen === 'day' ? chosen : started ? 'day' : 'before');
  function show(which: 'before' | 'day') {
    navigate(`/admin?view=${which}`, true);
  }
  const clientURL = $derived(lan.clientURL);
  const landingURL = $derived(clientURL ? `${clientURL}/` : '');
  // The score keepers' page, for the code a tablet scans (#138).
  const scoreURL = $derived(hallURL(clientURL) ? `${hallURL(clientURL)}/score` : '');
</script>

<main>
  <header>
    <h1>
      Porta di Ferro
      <span class="admintag">{t('admin')}</span>
      {#if view?.name}<span class="event">{view.name}</span>{/if}
    </h1>
    <nav>
      <a href="/plan">{t('Plan the event')}</a>
      <a href="/">{t('Landing page')}</a>
      <a href="/info">{t('Info sheet')}</a>
      <LangToggle />
    </nav>
  </header>

  {#if !view}
    <p class="loading">{hall.error || t('Loading…')}</p>
  {:else}
    {#if view.infoError}
      <p class="problem">
        {t('The event file could not be read, so the welcome, the programme and the wifi are blank until it is fixed. The disciplines run regardless.')}
        <span class="mono">{view.infoError}</span>
      </p>
    {/if}

    <section class="join">
      <div>
        <h2>{t('Everyone in the hall')}</h2>
        {#if landingURL}
          <p class="url">{landingURL}</p>
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
          {/if}
          <p class="hint">{t('One address for every discipline: the landing page, with each discipline’s results a tap away. It is what the info sheet prints.')}</p>
        {:else if demo}
          <p class="url none">{t('One tab, every screen')}</p>
        {:else}
          <p class="url none">{t('No network')}</p>
          <p class="hint warn">{t('This PC is not on a network another device could reach, so there is no address to hand out. Join it to the venue wifi and reload this page. Scoring on this PC still works.')}</p>
        {/if}

        <h3>{t('Each discipline')}</h3>
        <!-- The one list of the disciplines, with what can be done to each (#142): it was
             here and again in a box of its own further down. -->
        <Disciplines bare links />
      </div>
      {#if landingURL}
        <img class="qr" alt={t('QR code for {url}', { url: landingURL })} src={qrSrc(landingURL)} />
      {/if}
    </section>

    <!-- One address for every tablet: it picks a physical mat and follows it through
         whatever disciplines the plan puts there (phase 2). The code is for the tablets to
         scan (#138); a code per mat, printed, skips the question. -->
    <section class="join">
      <div>
        <h2>{t('Score keepers')}</h2>
        <p class="url">{scoreURL || '/score'}</p>
        <p class="hint">{t('Every score keeper’s tablet scans this, picks its mat once and follows it through every discipline.')}</p>
        <p class="links">
          <a href="/score">{t('Score keeper')}</a>
          <a href="/print/mats" target="_blank" rel="noreferrer">{t('Print a code for each mat')}</a>
          <a class="quiet" href="/display/mats" target="_blank" rel="noreferrer">{t('Displays')}</a>
        </p>
        <p class="links">
          {#each mats.view?.mats ?? [] as m (m.mat)}
            <a href="/display/mat/{m.mat}">{t('Display mat {n}', { n: m.mat })}</a>
          {/each}
        </p>
      </div>
      {#if scoreURL}
        <img class="qr" alt={t('QR code for {url}', { url: scoreURL })} src={qrSrc(scoreURL)} />
      {/if}
    </section>

    <div class="tabs" role="tablist" aria-label={t('Dashboard')}>
      <button role="tab" aria-selected={dashboard === 'before'} class:on={dashboard === 'before'} onclick={() => show('before')}>
        {t('Before the day')}
      </button>
      <button role="tab" aria-selected={dashboard === 'day'} class:on={dashboard === 'day'} onclick={() => show('day')}>
        {t('On the day')}
      </button>
    </div>

    {#if dashboard === 'day'}
      <MatBoard canSetCount />

      <Screens />
    {:else}
      <!-- One signup for the whole event, and the people it and the desks make (phase 3). -->
      <Signup scope="event" info={view.info} onchange={() => void hall.refresh()} />

      <!-- Only when two entries may be one person: the desks' doing, not the signup's. -->
      <People whenNeeded />

      <!-- Who works the mats, at event level (phase 5). -->
      <div id="staff"><Staff /></div>

      <DeleteEvent name={view.name} dir={view.dir} />
    {/if}

    <p class="dir">{t('The event’s files are in')} <span class="mono">{view.dir}</span></p>
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
    flex-wrap: wrap;
    align-items: baseline;
    gap: 0.6rem;
  }
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
  .event {
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
  .problem {
    background: var(--panel);
    border-left: 3px solid var(--amber-bright);
    border-radius: var(--radius);
    padding: 0.8rem 1rem;
    color: var(--amber-bright);
    overflow-wrap: anywhere;
  }
  .problem .mono {
    display: block;
    color: var(--ink-dim);
    margin-top: 0.3rem;
    font-size: 0.85rem;
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
  h3 {
    margin: 1rem 0 0.4rem;
    font-size: 0.95rem;
  }
  .url {
    margin: 0 0 0.5rem;
    font-size: clamp(1.2rem, 3vw, 1.8rem);
    font-weight: 700;
    font-variant-numeric: tabular-nums;
    overflow-wrap: anywhere;
  }
  .url.none {
    color: var(--ink-dim);
  }
  .on {
    margin: -0.3rem 0 0.6rem;
    color: var(--ink-dim);
    font-size: 0.95rem;
  }
  .network {
    display: inline-flex;
    align-items: center;
    gap: 0.6rem;
    margin-bottom: 0.6rem;
    font-size: 0.9rem;
    color: var(--ink-dim);
  }
  .hint {
    margin: 0 0 0.4rem;
    color: var(--ink-dim);
    line-height: 1.55;
    max-width: 40rem;
  }
  .hint.warn {
    color: var(--amber-bright);
  }
  .links {
    display: flex;
    flex-wrap: wrap;
    gap: 0.4rem 1rem;
    margin: 0;
    font-size: 0.9rem;
    overflow-wrap: anywhere;
  }
  .quiet {
    color: var(--ink-dim);
  }
  .qr {
    width: clamp(9rem, 22vw, 13rem);
    height: auto;
    background: #fff;
    padding: 0.5rem;
    border-radius: var(--radius);
  }
  .tabs {
    display: flex;
    gap: 0.4rem;
    margin: 0.4rem 0 1rem;
    border-bottom: 1px solid var(--line);
  }
  .tabs button {
    padding: 0.55rem 1rem;
    border: none;
    border-bottom: 3px solid transparent;
    border-radius: 0;
    background: none;
    color: var(--ink-dim);
    font-weight: 700;
  }
  .tabs button.on {
    color: var(--ink);
    border-bottom-color: var(--amber-bright);
  }
  .loading,
  .dir {
    color: var(--ink-dim);
  }
  .dir {
    font-size: 0.85rem;
    overflow-wrap: anywhere;
  }
</style>
