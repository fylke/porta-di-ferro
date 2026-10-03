<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '../api';
  import { hall } from '../lib/event.svelte';
  import { LanAddress, describeNetwork } from '../lib/lan.svelte';
  import Disciplines from './Disciplines.svelte';
  import MatBoard from './MatBoard.svelte';
  import Screens from './Screens.svelte';
  import EventEditor from './EventEditor.svelte';
  import People from './People.svelte';
  import Planning from './Planning.svelte';
  import Staff from './Staff.svelte';
  import Signup from './Signup.svelte';
  import LangToggle from './LangToggle.svelte';
  import { t } from '../lib/i18n.svelte';

  /**
   * The event's admin view, at /admin once the event runs more than one discipline
   * (docs/proposals/one-event-many-disciplines.md §6): what is about the event rather than
   * any one discipline. The disciplines -- add one, rename one, take one out -- and the day
   * around the fencing, typed once for the whole hall. Each discipline's competitors, pools
   * and matches are on its own admin page, one tap away.
   *
   * The join panel hands out the one address for everyone: the landing page, which leads
   * to every discipline. The score keepers' pages are per discipline until mats are shared
   * across them (phase 2), so each has its own line.
   */
  const lan = new LanAddress();
  const demo = import.meta.env.VITE_DEMO === 'true';

  onMount(() => {
    void hall.refresh();
    hall.follow();
    lan.start();
    return () => {
      lan.stop();
      hall.unfollow();
    };
  });

  const view = $derived(hall.view);
  const clientURL = $derived(lan.clientURL);
  const landingURL = $derived(clientURL ? `${clientURL}/` : '');
</script>

<main>
  <header>
    <h1>
      Porta di Ferro
      <span class="admintag">{t('admin')}</span>
      {#if view?.name}<span class="event">{view.name}</span>{/if}
    </h1>
    <nav>
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

        <!-- One address for every tablet: it picks a physical mat and follows it through
             whatever disciplines the plan puts there (phase 2). -->
        <h3>{t('Score keepers')}</h3>
        <p class="links">
          <a href="/score">{clientURL ? `${clientURL}/score` : '/score'}</a>
          <a class="quiet" href="/display/mats" target="_blank" rel="noreferrer">{t('Displays')}</a>
        </p>
        <h3>{t('Each discipline')}</h3>
        <ul class="perdiscipline">
          {#each view.disciplines.filter((d) => !d.error) as d (d.slug)}
            <li>
              <span class="dname">{d.name || t('Unnamed')}</span>
              <a href="/d/{d.slug}/admin">{t('Admin')}</a>
              <a class="quiet" href="/d/{d.slug}/">{t('Landing page')}</a>
            </li>
          {/each}
        </ul>
      </div>
      {#if landingURL}
        <img class="qr" alt={t('QR code for {url}', { url: landingURL })} src="/api/qr.png?url={encodeURIComponent(landingURL)}" />
      {/if}
    </section>

    <Disciplines />

    <MatBoard canSetCount />

    <Planning />

    <Screens />

    <!-- Seeded once and not re-keyed: the event updates on every exchange in every
         discipline, and an editor that reset itself on each would lose the sentence
         being typed. -->
    <EventEditor event={view.info} save={api.saveEventInfo} onchange={() => void hall.refresh()} />

    <!-- One signup for the whole event, and the people it and the desks make (phase 3). -->
    <Signup scope="event" info={view.info} onchange={() => void hall.refresh()} />

    <People />

    <!-- Who works the mats, at event level (phase 5). -->
    <Staff />


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
  .perdiscipline {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    gap: 0.3rem;
    font-size: 0.9rem;
  }
  .perdiscipline li {
    display: flex;
    flex-wrap: wrap;
    gap: 0.3rem 0.8rem;
    align-items: baseline;
  }
  .dname {
    font-weight: 700;
    min-width: 12rem;
  }
  .perdiscipline a {
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
  .loading,
  .dir {
    color: var(--ink-dim);
  }
  .dir {
    font-size: 0.85rem;
    overflow-wrap: anywhere;
  }
</style>
