<script lang="ts">
  import { api, type Presence, type Snapshot } from '../api';
  import { nameLookup } from './lib-display.svelte';

  /**
   * Who is connected: the screens and what each shows, the score keepers and which mat
   * and match each is on, whether each is still alive -- and anything set aside after a
   * handover (design §7 items 4 and 10).
   */
  let { presence, snapshot }: { presence: Presence | null; snapshot: Snapshot } = $props();

  const name = $derived(nameLookup(snapshot));
  const mats = $derived(Array.from({ length: snapshot.tournament.mats }, (_, i) => i + 1));
  const displays = $derived((presence?.clients ?? []).filter((c) => c.role === 'display'));
  const keepers = $derived((presence?.clients ?? []).filter((c) => c.role === 'scorekeeper'));

  // What a screen can be told to show. Short paths, the same ones the URLs use.
  const targets = $derived([
    { value: '', label: 'Nothing yet' },
    ...mats.map((m) => ({ value: `mat/${m}`, label: `Mat ${m} scoreboard` })),
    ...mats.map((m) => ({ value: `audience/${m}`, label: `Mat ${m} audience display` })),
    { value: 'mats', label: 'Every mat' },
    ...(mats.length > 2
      ? [
          { value: 'mats/1,2', label: 'Mats 1 and 2' },
          { value: `mats/${mats.slice(2).join(',')}`, label: `Mats ${mats.slice(2).join(' and ')}` },
        ]
      : []),
    { value: 'roster', label: 'Match roster' },
  ]);

  function label(target: string): string {
    return targets.find((t) => t.value === target)?.label ?? target;
  }

  function matchLabel(id: string | undefined): string {
    if (!id) return '';
    const m = snapshot.pools.flatMap((p) => p.matches).find((x) => x.id === id);
    return m ? `${name(m.red)} v ${name(m.blue)}` : id;
  }

  function ago(iso: string): string {
    const s = Math.max(0, Math.round((Date.now() - Date.parse(iso)) / 1000));
    return s < 60 ? `${s} s ago` : `${Math.round(s / 60)} min ago`;
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

  // Quarantined events grouped by match, so the organizer reads one line per incident.
  const setAside = $derived.by(() => {
    const groups = new Map<string, { match: string; clientName: string; count: number; at: string }>();
    for (const q of presence?.quarantined ?? []) {
      const g = groups.get(q.match) ?? { match: q.match, clientName: q.clientName, count: 0, at: q.receivedAt };
      g.count++;
      g.at = q.receivedAt;
      groups.set(q.match, g);
    }
    return [...groups.values()];
  });
</script>

<section>
  <h2>Screens and score keepers</h2>
  {#if error}<p class="err">{error}</p>{/if}

  <h3>Screens</h3>
  {#if displays.length === 0}
    <p class="dim">
      None yet. Open <span class="mono">/display</span> on a screen and it appears here,
      waiting to be told what to show.
    </p>
  {:else}
    <ul>
      {#each displays as d (d.id)}
        <li class:dead={!d.alive}>
          <span class="dot" title={d.alive ? 'Alive' : 'Not seen for a while'}></span>
          <span class="who">{d.name}</span>
          <select value={d.target ?? ''} onchange={(e) => void run(() => api.assignDisplay(d.id, e.currentTarget.value))}>
            {#each targets as t (t.value)}<option value={t.value}>{t.label}</option>{/each}
          </select>
          <span class="meta">{d.alive ? label(d.target ?? '') : `last seen ${ago(d.lastSeen)}`}</span>
        </li>
      {/each}
    </ul>
  {/if}

  <h3>Score keepers</h3>
  {#if keepers.length === 0}
    <p class="dim">None connected.</p>
  {:else}
    <ul>
      {#each keepers as k (k.id)}
        <li class:dead={!k.alive}>
          <span class="dot" title={k.alive ? 'Alive' : 'Not seen for a while'}></span>
          <span class="who">{k.name}</span>
          <span class="meta">
            {#if k.mat}Mat {k.mat}{/if}
            {#if k.match}&middot; {matchLabel(k.match)}{/if}
            {#if !k.alive}&middot; last seen {ago(k.lastSeen)}{/if}
          </span>
        </li>
      {/each}
    </ul>
  {/if}

  {#if setAside.length > 0}
    <h3>Set aside</h3>
    <p class="dim">
      Exchanges a device sent after its match had been handed to another. Not counted.
      Check the match log before discarding; a hand edit is the way to recover any of it.
    </p>
    <ul>
      {#each setAside as g (g.match)}
        <li>
          <span class="who">{g.count} exchange{g.count === 1 ? '' : 's'}</span>
          <span class="meta">from {g.clientName} on {matchLabel(g.match)}</span>
          <button onclick={() => void run(() => api.discardQuarantine(g.match))}>Discard</button>
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
