<script lang="ts">
  import { onMount } from 'svelte';
  import { hall } from '../lib/event.svelte';
  import { stageLabel } from '../lib/stage';
  import { t } from '../lib/i18n.svelte';
  import LangToggle from './LangToggle.svelte';
  import Schedule from './Schedule.svelte';
  import FencingProgramme from './FencingProgramme.svelte';

  /**
   * What everyone in the hall gets when the event runs several disciplines (#102).
   *
   * One address on the poster, one page for every phone: the welcome and the programme
   * once, every discipline side by side with where it has got to and what is on its mats,
   * and a search that finds a name in any of them. Each discipline's own landing page --
   * the pools, the standings, the bracket -- is one tap further, at /d/{discipline}/.
   *
   * It holds one stream for the whole event, which carries a short summary of each
   * discipline rather than every discipline's full snapshot, so a phone in the hall never
   * holds more than one (docs/proposals/one-event-many-disciplines.md §12). Read-only,
   * like the page it stands in front of.
   */
  onMount(() => {
    void hall.refresh();
    hall.follow();
    return () => hall.unfollow();
  });

  const view = $derived(hall.view);
  const info = $derived(view?.info ?? {});

  let query = $state('');
  const fold = (s: string) =>
    s
      .toLocaleLowerCase()
      .normalize('NFD')
      .replace(/[\u0300-\u036f]/g, '');
  // Somebody looking for their match knows their name, not which discipline the
  // organizer filed them under -- and may be in more than one, which is one person with
  // one page for the whole day (phase 3).
  const found = $derived.by(() => {
    const q = fold(query.trim());
    if (q.length < 2 || !view) return [];
    const byPerson = new Map<string, { key: string; href: string; name: string; club?: string; in: string[] }>();
    for (const d of view.disciplines) {
      for (const e of d.entrants) {
        if (!fold(e.name).includes(q) && !fold(e.club ?? '').includes(q)) continue;
        const key = e.person || `${d.slug}/${e.id}`;
        const row = byPerson.get(key);
        if (row) {
          row.in.push(d.name);
          continue;
        }
        byPerson.set(key, {
          key,
          href: e.person ? `/who/${e.person}` : `/d/${d.slug}/who/${e.id}`,
          name: e.name,
          club: e.club,
          in: [d.name],
        });
      }
    }
    return [...byPerson.values()].sort((a, b) => a.name.localeCompare(b.name)).slice(0, 30);
  });
</script>

<main>
  <header>
    <h1>{view?.name || 'Porta di Ferro'}</h1>
    <LangToggle />
  </header>

  {#if !view}
    <p class="loading">{hall.error || t('Loading…')}</p>
  {:else}
    <div class="layout">
      <section class="welcome">
        {#if info.welcome}
          <!-- The organizer's own words, from the admin view. Line breaks are theirs. -->
          <p class="intro">{info.welcome}</p>
        {:else}
          <p class="intro dim">{t('Welcome. The schedule and the results are on this page, and they update themselves.')}</p>
        {/if}
      </section>

      <!-- The side column on anything wider than a phone: the programme, and the search
           under it, stacked on their own rather than pinned to the rows beside them. -->
      <div class="side">
        <section class="schedule">
          <h2>{t('Programme')}</h2>
          <Schedule items={info.schedule ?? []} />
          <FencingProgramme rows={view.programme ?? []} />
        </section>

        <section class="find">
          <h2>{t('Find a name')}</h2>
          <input
            type="search"
            bind:value={query}
            placeholder={t('Your name, or your club')}
            aria-label={t('Find a name')}
          />
          {#if query.trim().length >= 2}
            {#if found.length === 0}
              <p class="dim">{t('Nobody by that name is entered.')}</p>
            {:else}
              <ul class="found">
                {#each found as f (f.key)}
                  <li>
                    <a href={f.href}>
                      <span class="who">{f.name}</span>
                      <span class="club">{f.club ?? ''}</span>
                      <span class="in">{f.in.join(', ')}</span>
                    </a>
                  </li>
                {/each}
              </ul>
            {/if}
          {/if}
        </section>
      </div>

      <section class="disciplines">
        <h2>{t('Disciplines')}</h2>
        <ul class="cards">
          {#each view.disciplines as d (d.slug)}
            <li class="card" class:failed={!!d.error}>
              <a class="title" href={d.url}>
                <span class="name">{d.name || t('Unnamed')}</span>
                <span class="go" aria-hidden="true">&rarr;</span>
              </a>
              {#if d.error && !d.stale}
                <p class="dim">{t('Not available just now.')}</p>
              {:else}
                {#if d.stale}
                  <p class="warn">{t('Not updating just now; this is how it last stood.')}</p>
                {/if}
                <p class="stage">
                  {stageLabel(d)}
                  {#if d.competitors > 0 && d.stage !== 'setup'}
                    <span class="dim">&middot; {t('{n} entered', { n: d.competitors })}</span>
                  {/if}
                </p>
                {#if d.matchesTotal > 0 && d.stage !== 'done'}
                  <div class="bar" role="presentation">
                    <span style="width: {(100 * d.matchesDone) / d.matchesTotal}%"></span>
                  </div>
                {/if}
                {#if d.podium && d.stage === 'done'}
                  <ol class="podium">
                    <li><span class="place">1</span> {d.podium.first}</li>
                    <li><span class="place">2</span> {d.podium.second}</li>
                    <li><span class="place">3</span> {d.podium.third}</li>
                  </ol>
                {:else}
                  <ul class="mats">
                    {#each d.mats.filter((m) => m.match) as m (m.mat)}
                      <li>
                        <a href="/display/mat/{m.mat}">
                          <span class="matname">{t('Mat {n}', { n: m.mat })}</span>
                          <span class="red" style="color: var(--bright-{m.redColour ?? 'red'})">{m.red}</span>
                          <span class="score mono">
                            {#if m.status === 'pending'}{t('up next')}{:else}{m.redScore}–{m.blueScore}{/if}
                          </span>
                          <span class="blue" style="color: var(--bright-{m.blueColour ?? 'blue'})">{m.blue}</span>
                        </a>
                      </li>
                    {/each}
                  </ul>
                {/if}
              {/if}
            </li>
          {/each}
        </ul>
      </section>
    </div>
  {/if}
</main>

<style>
  main {
    max-width: 78rem;
    margin: 0 auto;
    padding: 1rem 1rem 3rem;
  }
  header {
    display: flex;
    align-items: center;
    gap: 1rem;
    margin-bottom: 1rem;
  }
  h1 {
    flex: 1;
    margin: 0;
    font-size: clamp(1.25rem, 4vw, 1.8rem);
    min-width: 0;
    overflow-wrap: anywhere;
  }
  h2 {
    margin: 0 0 0.6rem;
    font-size: 0.78rem;
    font-weight: 800;
    letter-spacing: 0.12em;
    text-transform: uppercase;
    color: var(--ink-dim);
  }
  section {
    background: var(--panel);
    border-radius: var(--radius);
    padding: 1rem 1.1rem;
  }
  .loading,
  .dim {
    color: var(--ink-dim);
  }
  .warn {
    color: var(--amber-bright);
    font-size: 0.85rem;
    margin: 0 0 0.4rem;
  }
  .intro {
    margin: 0;
    line-height: 1.55;
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }

  /* One column on a phone, in reading order; the programme and the search move to a side
     column on anything wider, as on a discipline's own landing page. */
  .layout {
    display: grid;
    gap: 0.8rem;
    grid-template-columns: minmax(0, 1fr);
  }
  .layout > * {
    min-width: 0;
  }

  input[type='search'] {
    width: 100%;
    box-sizing: border-box;
  }
  .found {
    list-style: none;
    margin: 0.6rem 0 0;
    padding: 0;
    display: grid;
    gap: 1px;
  }
  .found a {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    gap: 0.1rem 0.6rem;
    padding: 0.5rem;
    border-radius: 6px;
    color: var(--ink);
    text-decoration: none;
  }
  .found a:hover {
    background: var(--panel-2);
  }
  .who {
    font-weight: 600;
    overflow-wrap: anywhere;
  }
  .club {
    color: var(--ink-dim);
    font-size: 0.85rem;
    text-align: right;
  }
  .in {
    grid-column: 1 / -1;
    color: var(--ink-dim);
    font-size: 0.78rem;
  }

  .cards {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    gap: 0.7rem;
    grid-template-columns: repeat(auto-fill, minmax(min(100%, 19rem), 1fr));
  }
  .card {
    background: var(--panel-2);
    border-radius: var(--radius);
    padding: 0.8rem 0.9rem;
    display: grid;
    gap: 0.45rem;
    align-content: start;
  }
  .card.failed {
    outline: 1px dashed var(--line);
  }
  .title {
    display: flex;
    align-items: baseline;
    gap: 0.5rem;
    color: var(--ink);
    text-decoration: none;
    font-weight: 800;
    font-size: 1.05rem;
  }
  .title .name {
    flex: 1;
    min-width: 0;
    overflow-wrap: anywhere;
  }
  .title:hover .name {
    text-decoration: underline;
  }
  .go {
    color: var(--ink-dim);
  }
  .stage {
    margin: 0;
    font-size: 0.9rem;
  }
  .bar {
    height: 0.3rem;
    border-radius: 999px;
    background: var(--panel);
    overflow: hidden;
  }
  .bar span {
    display: block;
    height: 100%;
    background: var(--amber-bright);
  }
  .podium {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    gap: 0.2rem;
    font-weight: 600;
  }
  .place {
    display: inline-block;
    width: 1.4rem;
    color: var(--amber-bright);
    font-weight: 800;
  }
  .mats {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    gap: 0.3rem;
  }
  .mats a {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr) auto minmax(0, 1fr);
    gap: 0.5rem;
    align-items: baseline;
    padding: 0.35rem 0.4rem;
    border-radius: 6px;
    color: var(--ink);
    text-decoration: none;
    font-size: 0.88rem;
  }
  .mats a:hover {
    background: var(--panel);
  }
  .matname {
    font-size: 0.7rem;
    font-weight: 800;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--ink-dim);
  }
  .mats .red {
    text-align: right;
    font-weight: 700;
    overflow-wrap: anywhere;
  }
  .mats .blue {
    font-weight: 700;
    overflow-wrap: anywhere;
  }
  .score {
    white-space: nowrap;
  }

  .side {
    display: grid;
    gap: 0.8rem;
    align-content: start;
    min-width: 0;
  }

  @media (min-width: 46rem) {
    .layout {
      grid-template-columns: minmax(0, 1.6fr) minmax(0, 1fr);
      align-items: start;
    }
    .welcome {
      grid-column: 1;
      grid-row: 1;
    }
    .disciplines {
      grid-column: 1;
      grid-row: 2;
    }
    .side {
      grid-column: 2;
      grid-row: 1 / span 2;
    }
  }
</style>
