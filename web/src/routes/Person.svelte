<script lang="ts">
  import { onMount } from 'svelte';
  import { api, type Competitor, type PersonView } from '../api';
  import { discipline, dhref, navigate } from '../router.svelte';
  import { hall } from '../lib/event.svelte';
  import { t } from '../lib/i18n.svelte';
  import LangToggle from './LangToggle.svelte';
  import PersonEntry from './PersonEntry.svelte';
  import { clockOf } from '../lib/clock-of-day';
  import { itemLabel, roleLabel } from '../lib/items';

  /**
   * One person's day (issue #98, and across the event since phase 3): every discipline
   * they entered, and in each every match they are in, with what happened in the ones that
   * are done.
   *
   * Reached by tapping a name on a landing page, and worth its own address: this is the
   * link a competitor sends their club, and the one a spectator keeps open to catch
   * somebody they came to watch. /who/pr-… is the person, across the event. /who/c7 is the
   * address from before people: a competitor of one discipline, which moves on to their
   * person as soon as it is known who that is.
   *
   * Read-only like the page it came from.
   */
  let { id }: { id: string } = $props();

  const isPerson = $derived(id.startsWith('pr-'));
  const cacheKey = $derived(`porta.person.${id}`);

  let person = $state<PersonView | null>(null);
  let error = $state('');
  let missing = $state(false);

  async function load() {
    try {
      const p = await api.person(id);
      // A link to somebody since merged into somebody else: their address now.
      if (p.id !== id) {
        navigate(`/who/${p.id}`, true);
        return;
      }
      person = p;
      error = '';
      missing = false;
      try {
        localStorage.setItem(cacheKey, JSON.stringify(p));
      } catch {
        // The live copy still works.
      }
    } catch (e) {
      if (e && typeof e === 'object' && 'status' in e && e.status === 404) missing = true;
      else error = e instanceof Error ? e.message : String(e);
    }
  }

  onMount(() => {
    if (!isPerson) return;
    try {
      const raw = localStorage.getItem(cacheKey);
      if (raw) person = JSON.parse(raw) as PersonView;
    } catch {
      // No cached copy.
    }
    // Their entries change when the organizer enters them somewhere else or merges two
    // people; the event's stream says when anything changed.
    hall.follow();
    return () => hall.unfollow();
  });

  // Who is entered where, and as whom: what this page is built from. A score coming in
  // changes the event's view too, and is no reason to ask again.
  const entered = $derived(
    (hall.view?.disciplines ?? [])
      .map((d) => d.slug + ':' + d.entrants.map((e) => `${e.id}=${e.person ?? ''}${e.name}`).join(','))
      .join('|'),
  );
  $effect(() => {
    if (!isPerson) return;
    void entered;
    void load();
  });

  // The old address: a competitor of one discipline. Their whole day is one step on.
  let legacy = $state<Competitor | null>(null);
  let legacyLoaded = $state(false);
  function onLegacy(entry: Competitor | null, loaded: boolean) {
    legacy = entry;
    legacyLoaded = loaded;
    if (entry?.person) navigate(`/who/${entry.person}`, true);
  }

  const headings = $derived(hall.multi || (person?.entries.length ?? 0) > 1);
  const withdrawnEverywhere = $derived(
    !!person && person.entries.length > 0 && person.entries.every((e) => e.withdrawn),
  );
</script>

<main>
  <header>
    <a class="back" href={isPerson ? '/' : dhref('/')}>&larr; {t('Everyone')}</a>
    <LangToggle />
  </header>

  {#if isPerson}
    {#if !person}
      {#if missing}
        <h1>{t('No such competitor')}</h1>
        <p class="dim">{t('Nobody by that link is in this event.')}</p>
      {:else}
        <p class="dim">{error || t('Loading…')}</p>
      {/if}
    {:else}
      <h1 class:out={withdrawnEverywhere}>{person.name}</h1>
      <p class="club">
        {person.club ?? ''}
        {#if withdrawnEverywhere && !headings}<span class="tag">{t('withdrawn')}</span>{/if}
      </p>
      {#if (person.duties?.length ?? 0) > 0 || person.physician}
        <!-- Their work as staff (phase 5), beside their fencing. -->
        <section class="duties">
          <h2>{t('Working')}</h2>
          {#if person.physician}<p class="oncall">{t('On call as physician all day.')}</p>{/if}
          <ol>
            {#each person.duties ?? [] as d (`${d.item}/${d.role}`)}
              <li>
                <span class="when mono">{clockOf(d.start)}–{clockOf(d.end)}</span>
                <span class="what">
                  <span class="strong">{roleLabel(d.role)}</span>
                  <span class="dim">&middot; {d.disciplineName} {itemLabel(d.kind, d.number).toLocaleLowerCase()} &middot; {t('mat {n}', { n: d.mat })}</span>
                </span>
              </li>
            {/each}
          </ol>
        </section>
      {/if}
      {#if person.entries.length === 0 && !(person.duties?.length ?? 0) && !person.physician}
        <p class="dim">{t('Not entered in anything just now.')}</p>
      {/if}
      {#each person.entries as e (`${e.discipline}/${e.competitor}`)}
        <PersonEntry slug={e.discipline} id={e.competitor} heading={headings ? e.disciplineName : ''} />
      {/each}
    {/if}
  {:else}
    {#if legacyLoaded && !legacy}
      <h1>{t('No such competitor')}</h1>
      <p class="dim">{t('That name is not in this tournament. It may be in another discipline.')}</p>
    {:else if legacy}
      <h1 class:out={legacy.withdrawn}>{legacy.name}</h1>
      <p class="club">
        {legacy.club}
        {#if legacy.withdrawn}<span class="tag">{t('withdrawn')}</span>{/if}
      </p>
    {/if}
    <PersonEntry slug={discipline()} {id} onentry={onLegacy} />
  {/if}
</main>

<style>
  main {
    max-width: 44rem;
    margin: 0 auto;
    padding: 1rem 1rem 3rem;
  }
  header {
    display: flex;
    align-items: center;
    gap: 1rem;
    margin-bottom: 0.8rem;
  }
  .back {
    flex: 1;
    color: var(--ink-dim);
    text-decoration: none;
    font-size: 0.9rem;
  }
  .back:hover {
    color: var(--ink);
  }
  h1 {
    margin: 0;
    font-size: clamp(1.4rem, 5vw, 2rem);
    overflow-wrap: anywhere;
  }
  h1.out {
    text-decoration: line-through;
    opacity: 0.6;
  }
  .club {
    margin: 0.2rem 0 1rem;
    color: var(--ink-dim);
  }
  .tag {
    margin-left: 0.5rem;
    font-size: 0.72rem;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--amber-bright);
  }
  .dim {
    color: var(--ink-dim);
  }
  .duties {
    background: var(--panel);
    border-radius: var(--radius);
    padding: 1rem 1.1rem;
    margin-bottom: 1.2rem;
  }
  .duties h2 {
    margin: 0 0 0.6rem;
    font-size: 0.78rem;
    font-weight: 800;
    letter-spacing: 0.12em;
    text-transform: uppercase;
    color: var(--ink-dim);
  }
  .duties ol {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    gap: 0.35rem;
  }
  .duties li {
    display: grid;
    grid-template-columns: 6.5rem minmax(0, 1fr);
    gap: 0.5rem;
    align-items: baseline;
  }
  .duties .when {
    color: var(--ink-dim);
    font-variant-numeric: tabular-nums;
  }
  .duties .what {
    overflow-wrap: anywhere;
  }
  .strong {
    font-weight: 700;
  }
  .oncall {
    margin: 0 0 0.5rem;
  }
</style>
