<script lang="ts">
  import { untrack } from 'svelte';
  import { api, type Snapshot } from '../api';

  /**
   * Tournament setup: mats, the pool size range, and the draw. Name, logo and discipline
   * linkage are Milestone 3 -- one run of the application is one tournament under one
   * hardcoded ruleset (design §9, issue #2).
   */
  let { snapshot, onchange }: { snapshot: Snapshot; onchange: () => void } = $props();

  // Editable copies. Deliberately seeded once rather than following the snapshot: the
  // organizer is the only writer, and a field that rewrites itself under the cursor while
  // they are typing would be worse than one that is briefly out of date.
  let mats = $state(untrack(() => snapshot.tournament.mats));
  let minPoolSize = $state(untrack(() => snapshot.tournament.minPoolSize));
  let maxPoolSize = $state(untrack(() => snapshot.tournament.maxPoolSize));
  let error = $state('');
  let busy = $state(false);

  const drawn = $derived(snapshot.pools.length > 0);
  const scored = $derived(snapshot.pools.some((p) => p.matches.some((m) => m.status !== 'pending')));

  async function save() {
    error = '';
    busy = true;
    try {
      await api.saveTournament(mats, minPoolSize, maxPoolSize);
      onchange();
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    } finally {
      busy = false;
    }
  }

  async function draw() {
    error = '';
    busy = true;
    try {
      await api.saveTournament(mats, minPoolSize, maxPoolSize);
      await api.generatePools();
      onchange();
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    } finally {
      busy = false;
    }
  }
</script>

<section>
  <h2>Tournament</h2>

  <div class="fields">
    <label>
      Mats
      <select bind:value={mats} onchange={save}>
        <option value={1}>1</option>
        <option value={2}>2</option>
      </select>
    </label>
    <label>
      Smallest pool
      <input type="number" min="2" max="7" bind:value={minPoolSize} onchange={save} />
    </label>
    <label>
      Largest pool
      <input type="number" min="2" max="7" bind:value={maxPoolSize} onchange={save} />
    </label>
  </div>

  <button class="draw" disabled={busy} onclick={draw}>
    {drawn ? 'Draw the pools again' : 'Draw the pools'}
  </button>

  {#if scored}
    <p class="warn">
      Matches have already been scored. Drawing again replaces the pools and the results
      stop lining up with them.
    </p>
  {/if}
  {#if error}<p class="err">{error}</p>{/if}
  {#each snapshot.tournament.violations ?? [] as v (v)}
    <p class="warn">{v}</p>
  {/each}

  <dl>
    <dt>Ruleset</dt>
    <dd>MSL, hardcoded &middot; 8 points &middot; 3 minutes &middot; differential scoring</dd>
    <dt>Ceiling</dt>
    <dd>2 mats, 4 pools of up to 7 &mdash; 28 competitors</dd>
    <dt>Data</dt>
    <dd class="path">{snapshot.dir}</dd>
  </dl>
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
  }
  .fields {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 0.6rem;
    margin-bottom: 0.9rem;
  }
  label {
    display: grid;
    gap: 0.3rem;
    font-size: 0.85rem;
    color: var(--ink-dim);
  }
  .draw {
    width: 100%;
    padding: 0.85rem;
    font-weight: 700;
    background: var(--panel-2);
    border: 2px solid var(--line);
  }
  .warn,
  .err {
    margin: 0.8rem 0 0;
    font-size: 0.88rem;
    line-height: 1.5;
    color: var(--amber-bright);
  }
  dl {
    margin: 1.1rem 0 0;
    display: grid;
    grid-template-columns: auto 1fr;
    gap: 0.35rem 0.9rem;
    font-size: 0.85rem;
  }
  dt {
    color: var(--ink-dim);
  }
  dd {
    margin: 0;
  }
  .path {
    word-break: break-all;
    color: var(--ink-dim);
  }
</style>
