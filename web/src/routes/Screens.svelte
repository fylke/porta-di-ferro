<script lang="ts">
  import { onMount } from 'svelte';
  import { api, apiIn } from '../api';
  import { hall } from '../lib/event.svelte';
  import { MatsLive } from '../lib/mats.svelte';
  import { t } from '../lib/i18n.svelte';

  /**
   * Who is connected: the screens and what each shows, the score keepers and which mat
   * and match each is on, whether each is still alive -- and anything set aside after a
   * handover (design §7 items 4 and 10).
   *
   * The devices are the event's (phase 2): a score keeper sits at a physical mat and a
   * screen shows one, whatever discipline is on it, so this panel follows the event's
   * stream and the hall's mats rather than any one discipline's.
   */
  const mats = new MatsLive();
  onMount(() => {
    void mats.start();
    void hall.refresh();
    hall.follow();
    api
      .presence()
      .then((p) => (hall.presence = p))
      .catch(() => {
        // The stream brings it along.
      });
    return () => {
      mats.stop();
      hall.unfollow();
    };
  });

  const presence = $derived(hall.presence);
  const matNumbers = $derived((mats.view?.mats ?? []).map((m) => m.mat));
  const disciplines = $derived(hall.view?.disciplines ?? []);
  const displays = $derived((presence?.clients ?? []).filter((c) => c.role === 'display'));
  const keepers = $derived((presence?.clients ?? []).filter((c) => c.role === 'scorekeeper'));

  // What a screen can be told to show. Short paths, the same ones the URLs use. The mats
  // are the hall's; a roster is a discipline's, so with several there is one each.
  const targets = $derived([
    { value: '', label: t('Nothing yet') },
    ...matNumbers.map((m) => ({ value: `mat/${m}`, label: t('Mat {n} scoreboard', { n: m }) })),
    ...matNumbers.map((m) => ({ value: `audience/${m}`, label: t('Mat {n} audience display', { n: m }) })),
    { value: 'mats', label: t('Every mat') },
    // A screen between two mats: the pairs, when there are pairs to make.
    ...(matNumbers.length >= 3 ? [{ value: 'mats/1,2', label: t('Mats {a} and {b}', { a: 1, b: 2 }) }] : []),
    ...(matNumbers.length === 4 ? [{ value: 'mats/3,4', label: t('Mats {a} and {b}', { a: 3, b: 4 }) }] : []),
    ...(disciplines.length > 1
      ? disciplines.map((d) => ({ value: `d/${d.slug}/roster`, label: t('Match roster: {name}', { name: d.name || d.slug }) }))
      : [{ value: 'roster', label: t('Match roster') }]),
  ]);

  function label(target: string): string {
    return targets.find((t) => t.value === target)?.label ?? target;
  }

  /** "Astrid v Bo", from the mat the score keeper is at. */
  function matchLabel(discipline: string | undefined, id: string | undefined): string {
    if (!id) return '';
    for (const m of mats.view?.mats ?? []) {
      const s = m.queue.find((q) => q.match.id === id && (!discipline || q.discipline === discipline));
      if (s) return `${s.red} v ${s.blue}`;
    }
    return id;
  }

  function ago(iso: string): string {
    const s = Math.max(0, Math.round((Date.now() - Date.parse(iso)) / 1000));
    return s < 60 ? t('{n} s ago', { n: s }) : t('{n} min ago', { n: Math.round(s / 60) });
  }

  let error = $state('');
  async function run(fn: () => Promise<unknown>) {
    error = '';
    try {
      await fn();
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    }
  }

  // Quarantined events grouped by match, so the organizer reads one line per incident. A
  // match is a discipline's, so the discipline is part of what makes it one incident.
  const setAside = $derived.by(() => {
    const groups = new Map<string, { discipline: string; match: string; clientName: string; count: number; at: string }>();
    for (const q of presence?.quarantined ?? []) {
      const key = `${q.discipline ?? ''}/${q.match}`;
      const g = groups.get(key) ?? { discipline: q.discipline ?? '', match: q.match, clientName: q.clientName, count: 0, at: q.receivedAt };
      g.count++;
      g.at = q.receivedAt;
      groups.set(key, g);
    }
    return [...groups.values()];
  });
</script>

<section>
  <h2>{t('Screens and score keepers')}</h2>
  {#if error}<p class="err">{error}</p>{/if}

  <h3>{t('Screens')}</h3>
  {#if displays.length === 0}
    <p class="dim">
      {t('None yet. Open')} <span class="mono">/display</span> {t('on a screen and it appears here, waiting to be told what to show.')}
    </p>
  {:else}
    <ul>
      {#each displays as d (d.id)}
        <li class:dead={!d.alive}>
          <span class="dot" title={d.alive ? t('Alive') : t('Not seen for a while')}></span>
          <span class="who">{d.name}</span>
          <select value={d.target ?? ''} onchange={(e) => void run(() => api.assignDisplay(d.id, e.currentTarget.value))}>
            {#each targets as opt (opt.value)}<option value={opt.value}>{opt.label}</option>{/each}
          </select>
          <span class="meta">{d.alive ? label(d.target ?? '') : t('last seen {when}', { when: ago(d.lastSeen) })}</span>
        </li>
      {/each}
    </ul>
  {/if}

  <h3>{t('Score keepers')}</h3>
  {#if keepers.length === 0}
    <p class="dim">{t('None connected.')}</p>
  {:else}
    <ul>
      {#each keepers as k (k.id)}
        <li class:dead={!k.alive}>
          <span class="dot" title={k.alive ? t('Alive') : t('Not seen for a while')}></span>
          <span class="who">{k.name}</span>
          <span class="meta">
            {#if k.mat}{t('Mat {n}', { n: k.mat })}{/if}
            {#if k.match}&middot; {matchLabel(k.discipline, k.match)}{/if}
            {#if !k.alive}&middot; {t('last seen {when}', { when: ago(k.lastSeen) })}{/if}
          </span>
        </li>
      {/each}
    </ul>
  {/if}

  {#if setAside.length > 0}
    <h3>{t('Set aside')}</h3>
    <p class="dim">{t('Exchanges a device sent after its match had been handed to another. Not counted. Check the match log before discarding; a hand edit is the way to recover any of it.')}</p>
    <ul>
      {#each setAside as g (`${g.discipline}/${g.match}`)}
        <li>
          <span class="who">{g.count === 1 ? t('1 exchange') : t('{n} exchanges', { n: g.count })}</span>
          <span class="meta">{t('from {device} on {match}', { device: g.clientName, match: matchLabel(g.discipline, g.match) })}</span>
          <button onclick={() => void run(() => apiIn(g.discipline).discardQuarantine(g.match))}>{t('Discard')}</button>
        </li>
      {/each}
    </ul>
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
    margin: 0 0 0.6rem;
    font-size: 1.15rem;
  }
  h3 {
    margin: 0.9rem 0 0.4rem;
    font-size: 0.75rem;
    font-weight: 800;
    letter-spacing: 0.1em;
    text-transform: uppercase;
    color: var(--ink-dim);
  }
  ul {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    gap: 0.35rem;
  }
  li {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.6rem;
    font-size: 0.92rem;
  }
  li.dead {
    opacity: 0.55;
  }
  .dot {
    width: 0.6rem;
    height: 0.6rem;
    border-radius: 50%;
    background: var(--ok);
    flex: none;
  }
  li.dead .dot {
    background: var(--ink-dim);
  }
  .who {
    font-weight: 700;
  }
  .meta {
    color: var(--ink-dim);
  }
  select {
    padding: 0.3rem 0.5rem;
    font-size: 0.85rem;
  }
  button {
    padding: 0.3rem 0.7rem;
    font-size: 0.85rem;
    background: var(--panel-2);
    border: 1px solid var(--line);
  }
  .dim {
    margin: 0;
    color: var(--ink-dim);
    line-height: 1.5;
    font-size: 0.9rem;
  }
  .err {
    color: var(--amber-bright);
  }
</style>
