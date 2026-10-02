<script lang="ts">
  import { onMount } from 'svelte';
  import { api, type Competitor } from '../api';
  import { hall } from '../lib/event.svelte';
  import { discipline } from '../router.svelte';
  import { t } from '../lib/i18n.svelte';
  import { fitRows, type Layout } from '../lib/fit';

  /**
   * Registration and status. Name and club only -- picture, phone number and club crest
   * are Milestone 3, and push notifications are out entirely: they need internet, which a
   * LAN-only server does not have (design §9, issue #1).
   */
  let { competitors, poolsDrawn, onchange, multi = false }: {
    competitors: Competitor[];
    poolsDrawn: boolean;
    onchange: () => void;
    /** The event runs several disciplines, so somebody typed in may be entered elsewhere. */
    multi?: boolean;
  } = $props();

  // A fresh look at who is entered elsewhere, for the suggestions below.
  onMount(() => {
    if (multi) void hall.refresh();
  });

  const fold = (s: string) =>
    s
      .toLocaleLowerCase()
      .normalize('NFD')
      .replace(/[\u0300-\u036f]/g, '')
      .replace(/[\s-]+/g, ' ')
      .trim();

  /**
   * Somebody entered in another discipline with the name being typed (phase 3). The desk
   * picks one -- this is them -- or adds somebody new; a name alone never makes two
   * entries one person.
   */
  const suggestions = $derived.by(() => {
    const q = fold(name);
    if (!multi || q.length < 2) return [];
    const here = new Set(competitors.map((c) => c.person).filter(Boolean));
    const out = new Map<string, { person: string; name: string; club?: string; in: string[] }>();
    for (const d of hall.view?.disciplines ?? []) {
      if (d.slug === discipline()) continue;
      for (const e of d.entrants) {
        if (!e.person || here.has(e.person) || !fold(e.name).startsWith(q)) continue;
        const row = out.get(e.person);
        if (row) row.in.push(d.name);
        else out.set(e.person, { person: e.person, name: e.name, club: e.club, in: [d.name] });
      }
    }
    return [...out.values()].slice(0, 5);
  });

  let name = $state('');
  let club = $state('');
  let error = $state('');

  const active = $derived(competitors.filter((c) => !c.withdrawn).length);

  // A club open of up to 32 is read in one go, and a box that scrolls inside a page that
  // also scrolls is two scrollbars for one list. Past that the list would push everything
  // under it off the screen, so it scrolls in place.
  const SCROLL_FROM = 33;

  let layout = $state<Layout>('one');

  async function add(event: SubmitEvent) {
    event.preventDefault();
    await enter();
  }

  /** person: somebody entered elsewhere this is, picked from the suggestions. */
  async function enter(person?: { person: string; name: string; club?: string }) {
    error = '';
    // What was sent, so the field is cleared only if it still says that. A volunteer at the
    // desk types the next name while this one is saving, and clearing the field when the
    // answer came wiped what they had typed since.
    const sent = name;
    try {
      if (person) await api.addCompetitor(person.name, person.club ?? club, person.person);
      else await api.addCompetitor(sent, club);
      if (name === sent) name = '';
      // The club usually repeats down a queue of people signing in together, so it stays.
      onchange();
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    }
  }

  async function setWithdrawn(c: Competitor, withdrawn: boolean) {
    error = '';
    try {
      await api.updateCompetitor(c.id, { withdrawn });
      onchange();
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    }
  }

  async function remove(c: Competitor) {
    error = '';
    try {
      await api.removeCompetitor(c.id);
      onchange();
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    }
  }
</script>

<section>
  <h2>{t('Competitors')} <span class="count">{t('{n} entered', { n: active })}</span></h2>

  <form onsubmit={add}>
    <input
      bind:value={name}
      placeholder={t('Name')}
      required
      aria-label={t('Competitor name')}
      onfocus={() => multi && void hall.refresh()}
    />
    <input bind:value={club} placeholder={t('Club')} aria-label={t('Club')} />
    <button type="submit">{t('Add')}</button>
  </form>
  {#if suggestions.length > 0}
    <div class="suggest">
      <p class="dim-small">{t('Already entered elsewhere. Is this them?')}</p>
      <ul class="suggestions">
        {#each suggestions as s (s.person)}
          <li>
            <button type="button" class="pick" onclick={() => void enter(s)}>
              <span class="name">{s.name}</span>
              <span class="club">{s.club ?? ''}</span>
              <span class="in">{s.in.join(', ')}</span>
            </button>
          </li>
        {/each}
      </ul>
      <p class="dim-small">{t('If not, Add enters somebody new.')}</p>
    </div>
  {/if}
  {#if error}<p class="err">{error}</p>{/if}

  <ul
    class:scroll={competitors.length >= SCROLL_FROM}
    data-layout={layout}
    use:fitRows={{ onlayout: (l) => (layout = l) }}
  >
    {#each competitors as c (c.id)}
      <li class="fit-row" class:withdrawn={c.withdrawn}>
        <span class="name">{c.name}</span>
        <span class="club">{c.club}</span>
        <span class="actions">
          {#if c.withdrawn}
            <span class="tag">{t('Withdrawn')}</span>
            <button class="link" onclick={() => setWithdrawn(c, false)}>{t('Reinstate')}</button>
          {:else if poolsDrawn}
            <button class="link" onclick={() => setWithdrawn(c, true)}>{t('Withdraw')}</button>
          {:else}
            <button class="link" onclick={() => remove(c)}>{t('Remove')}</button>
          {/if}
        </span>
      </li>
    {/each}
  </ul>
  {#if poolsDrawn}
    <p class="hint">{t("Pools are drawn, so competitors can only be withdrawn from here. A withdrawal voids their results as though they never entered, and everyone else's standings recompute.")}</p>
  {/if}
</section>

<style>
  section {
    background: var(--panel);
    border-radius: var(--radius);
    padding: 1.1rem 1.25rem;
  }
  h2 {
    margin: 0 0 0.9rem;
    font-size: 1.15rem;
    display: flex;
    justify-content: space-between;
    align-items: baseline;
  }
  .count {
    font-size: 0.85rem;
    font-weight: 400;
    color: var(--ink-dim);
  }
  form {
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(0, 1fr) auto;
    gap: 0.5rem;
    margin-bottom: 0.9rem;
  }
  form button {
    padding: 0.55rem 1.1rem;
    font-weight: 700;
    background: var(--panel-2);
    border: 1px solid var(--line);
  }
  ul {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    gap: 0.2rem;
  }
  ul.scroll {
    max-height: 22rem;
    overflow-y: auto;
  }
  li {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto auto;
    gap: 0.6rem;
    align-items: baseline;
    padding: 0.4rem 0.5rem;
    border-radius: 6px;
  }
  .actions {
    display: inline-flex;
    gap: 0.6rem;
    align-items: baseline;
    justify-self: end;
  }
  /* Every row on one line at its natural width while it is measured; the widest decides
     whether all of them get one line or two (lib/fit.ts). */
  ul:global(.fit-measure) li.fit-row {
    display: flex;
    width: max-content;
    white-space: nowrap;
  }
  /* Too wide for one line: name over club in every row, never only in some. */
  ul[data-layout='stacked'] li {
    grid-template-columns: minmax(0, 1fr) auto;
    gap: 0.1rem 0.6rem;
  }
  ul[data-layout='stacked'] .name {
    grid-column: 1;
    grid-row: 1;
  }
  ul[data-layout='stacked'] .club {
    grid-column: 1;
    grid-row: 2;
  }
  ul[data-layout='stacked'] .actions {
    grid-column: 2;
    grid-row: 1 / span 2;
    align-self: center;
  }
  ul[data-layout='stacked'] .name,
  ul[data-layout='stacked'] .club {
    overflow-wrap: anywhere;
  }
  li:nth-child(odd) {
    background: var(--panel-2);
  }
  li.withdrawn .name,
  li.withdrawn .club {
    text-decoration: line-through;
    color: var(--ink-dim);
  }
  .club {
    color: var(--ink-dim);
    font-size: 0.9rem;
  }
  .tag {
    font-size: 0.75rem;
    color: var(--amber-bright);
  }
  .link {
    background: none;
    border: none;
    color: var(--blue-bright);
    font-size: 0.85rem;
    padding: 0;
  }
  .suggest {
    margin: -0.4rem 0 0.9rem;
    padding: 0.6rem 0.7rem;
    border-radius: 6px;
    border: 1px dashed var(--line);
  }
  .dim-small {
    margin: 0 0 0.4rem;
    color: var(--ink-dim);
    font-size: 0.82rem;
  }
  .suggestions {
    margin-bottom: 0.4rem;
  }
  .suggestions li {
    display: block;
    padding: 0;
  }
  .pick {
    display: flex;
    flex-wrap: wrap;
    gap: 0.2rem 0.6rem;
    align-items: baseline;
    width: 100%;
    text-align: left;
    padding: 0.4rem 0.5rem;
    background: var(--panel-2);
    border: 1px solid var(--line);
    border-radius: 6px;
    color: var(--ink);
  }
  .pick .name {
    font-weight: 700;
  }
  .pick .in {
    margin-left: auto;
    color: var(--ink-dim);
    font-size: 0.82rem;
  }
  .hint,
  .err {
    margin: 0.9rem 0 0;
    font-size: 0.9rem;
    line-height: 1.5;
    color: var(--ink-dim);
  }
  .err {
    color: var(--amber-bright);
  }
</style>
