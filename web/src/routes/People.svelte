<script lang="ts">
  import { api, type PeopleView, type PersonView } from '../api';
  import { hall } from '../lib/event.svelte';
  import { t } from '../lib/i18n.svelte';

  /**
   * The event's people (phase 3), for the organizer: everybody entered anywhere, and the
   * ones who might be one person.
   *
   * The app reports and the organizer decides. A signup's submission id already made one
   * response one person in every discipline it entered; a name never does on its own, so
   * two entries with the same name -- "Karl-Johan" and "Karl Johan", or two Anna Nilssons
   * -- are asked about here: the same person, or two. A merge can be undone, and two kept
   * apart stop being asked about.
   */
  let people = $state<PeopleView | null>(null);
  let error = $state('');
  let busy = $state(false);

  async function load() {
    try {
      people = await api.people();
      error = '';
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    }
  }

  // Who is entered where, and as whom: a new entry or a merge anywhere changes it, and a
  // score coming in does not.
  const entered = $derived(
    (hall.view?.disciplines ?? [])
      .map((d) => d.slug + ':' + d.entrants.map((e) => `${e.id}=${e.person ?? ''}${e.name}`).join(','))
      .join('|'),
  );
  $effect(() => {
    void entered;
    void load();
  });

  const byId = $derived(new Map((people?.people ?? []).map((p) => [p.id, p])));

  /** Every pair in a group still to decide. Two is the usual; three is possible. */
  const pairs = $derived.by(() => {
    const out: { key: string; a: PersonView; b: PersonView }[] = [];
    for (const group of people?.duplicates ?? []) {
      const members = group.map((id) => byId.get(id)).filter((p): p is PersonView => !!p);
      for (let i = 0; i < members.length; i++) {
        for (let j = i + 1; j < members.length; j++) {
          out.push({ key: `${members[i].id}+${members[j].id}`, a: members[i], b: members[j] });
        }
      }
    }
    return out;
  });

  const merged = $derived((people?.people ?? []).filter((p) => (p.mergedFrom?.length ?? 0) > 0));

  async function act(f: () => Promise<PeopleView>) {
    busy = true;
    error = '';
    try {
      people = await f();
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    } finally {
      busy = false;
    }
  }

  function where(p: PersonView): string {
    return p.entries.map((e) => e.disciplineName).join(', ');
  }
</script>

<section>
  <h2>
    {t('People')}
    {#if people}<span class="count">{t('{n} entered', { n: people.people.length })}</span>{/if}
  </h2>
  <p class="dim">
    {t('One person, however many disciplines they entered: one page with their whole day. A signup makes one response one person everywhere; the same name alone never does, so it is asked about here.')}
  </p>
  {#if error}<p class="err">{error}</p>{/if}

  {#if pairs.length > 0}
    <h3>{t('Maybe the same person')}</h3>
    <ul class="pairs">
      {#each pairs as pair (pair.key)}
        <li>
          <span class="who">
            <a href="/who/{pair.a.id}" target="_blank" rel="noreferrer">{pair.a.name}</a>
            <span class="dim-inline">{pair.a.club ?? ''} &middot; {where(pair.a)}</span>
          </span>
          <span class="who">
            <a href="/who/{pair.b.id}" target="_blank" rel="noreferrer">{pair.b.name}</a>
            <span class="dim-inline">{pair.b.club ?? ''} &middot; {where(pair.b)}</span>
          </span>
          <span class="actions">
            <button class="save" disabled={busy} onclick={() => act(() => api.mergePerson(pair.b.id, pair.a.id))}>
              {t('The same person')}
            </button>
            <button class="quiet" disabled={busy} onclick={() => act(() => api.keepApart(pair.a.id, pair.b.id))}>
              {t('Two people')}
            </button>
          </span>
        </li>
      {/each}
    </ul>
  {:else if people}
    <p class="ok">{t('Nobody to ask about: no two people share a name.')}</p>
  {/if}

  {#if merged.length > 0}
    <h3>{t('Merged')}</h3>
    <ul class="merged">
      {#each merged as p (p.id)}
        {#each p.mergedFrom ?? [] as from (from.id)}
          <li>
            <span>{t('{from} is now {into}', { from: from.name, into: p.name })}</span>
            <button class="quiet" disabled={busy} onclick={() => act(() => api.unmergePerson(from.id))}>
              {t('Separate again')}
            </button>
          </li>
        {/each}
      {/each}
    </ul>
  {/if}

  {#if people && people.people.length > 0}
    <details>
      <summary>{t('Everybody')}</summary>
      <ul class="everybody">
        {#each people.people as p (p.id)}
          <li>
            <a href="/who/{p.id}" target="_blank" rel="noreferrer">{p.name}</a>
            <span class="dim-inline">{p.club ?? ''}</span>
            <span class="in">{where(p)}</span>
          </li>
        {/each}
      </ul>
    </details>
  {/if}
</section>

<style>
  section {
    background: var(--panel);
    border-radius: var(--radius);
    padding: 1.1rem 1.25rem;
    margin-bottom: 1rem;
  }
  h2 {
    margin: 0 0 0.5rem;
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
  h3 {
    margin: 1rem 0 0.4rem;
    font-size: 0.72rem;
    font-weight: 800;
    letter-spacing: 0.1em;
    text-transform: uppercase;
    color: var(--ink-dim);
  }
  .dim {
    color: var(--ink-dim);
    line-height: 1.5;
    font-size: 0.9rem;
    margin: 0 0 0.6rem;
    max-width: 48rem;
  }
  .dim-inline {
    color: var(--ink-dim);
    font-size: 0.85rem;
    margin-left: 0.4rem;
  }
  .ok {
    color: var(--ok);
    font-size: 0.9rem;
    margin: 0.4rem 0;
  }
  .err {
    color: var(--amber-bright);
    font-size: 0.9rem;
  }
  ul {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    gap: 0.3rem;
  }
  .pairs li {
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(0, 1fr) auto;
    gap: 0.4rem 1rem;
    align-items: center;
    padding: 0.5rem 0.6rem;
    border-radius: 6px;
    background: var(--panel-2);
  }
  @media (max-width: 640px) {
    .pairs li {
      grid-template-columns: 1fr;
    }
  }
  .who {
    min-width: 0;
    overflow-wrap: anywhere;
  }
  .who a {
    font-weight: 700;
  }
  .actions {
    display: flex;
    flex-wrap: wrap;
    gap: 0.4rem;
  }
  .save,
  .quiet {
    padding: 0.4rem 0.8rem;
    font-size: 0.85rem;
    font-weight: 700;
    border-radius: var(--radius);
  }
  .save {
    background: var(--ink);
    color: #0d0f14;
    border: none;
  }
  .quiet {
    background: var(--panel-2);
    color: var(--ink);
    border: 1px solid var(--line);
  }
  .merged li {
    display: flex;
    flex-wrap: wrap;
    gap: 0.4rem 0.8rem;
    align-items: center;
    font-size: 0.9rem;
  }
  details {
    margin-top: 1rem;
  }
  summary {
    cursor: pointer;
    color: var(--ink-dim);
    font-size: 0.9rem;
  }
  .everybody {
    margin-top: 0.5rem;
    max-height: 22rem;
    overflow-y: auto;
  }
  .everybody li {
    display: flex;
    flex-wrap: wrap;
    gap: 0.2rem 0.6rem;
    align-items: baseline;
    font-size: 0.9rem;
    padding: 0.2rem 0;
    border-bottom: 1px solid var(--line);
  }
  .everybody .in {
    margin-left: auto;
    color: var(--ink-dim);
    font-size: 0.82rem;
  }
</style>
