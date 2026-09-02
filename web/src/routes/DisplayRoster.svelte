<script lang="ts">
  import { onMount } from 'svelte';
  import { keepAwake } from '../lib/wakelock';
  import { Live, nameLookup } from './lib-display.svelte';

  /** Every match in the event and where it stands. The screen competitors check. */
  const live = new Live();

  onMount(() => {
    live.start();
    const release = keepAwake();
    return () => {
      live.stop();
      release();
    };
  });

  const name = $derived(nameLookup(live.snapshot));
</script>

<main>
  <h1>Match roster</h1>
  <div class="pools">
    {#each live.snapshot?.pools ?? [] as pool (pool.number)}
      <section>
        <h2>Pool {pool.number} <span class="mat">Mat {pool.mat}</span></h2>
        <ol>
          {#each pool.matches as m (m.id)}
            <li class={m.status}>
              <span class="n mono">{m.order}</span>
              <span class="red">{name(m.red)}</span>
              <span class="score mono">
                {#if m.status === 'pending'}v{:else}{m.state.red.score}–{m.state.blue.score}{/if}
              </span>
              <span class="blue">{name(m.blue)}</span>
            </li>
          {/each}
        </ol>
      </section>
    {/each}
    {#if (live.snapshot?.pools ?? []).length === 0}
      <p class="empty">Pools have not been drawn yet.</p>
    {/if}
  </div>
</main>

<style>
  main {
    min-height: 100dvh;
    padding: 1.25rem;
  }
  h1 {
    margin: 0 0 1rem;
    font-size: clamp(1.3rem, 4vh, 2.2rem);
  }
  .pools {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(19rem, 1fr));
    gap: 1rem;
    align-items: start;
  }
  section {
    background: var(--panel);
    border-radius: var(--radius);
    padding: 0.9rem 1rem;
  }
  h2 {
    margin: 0 0 0.6rem;
    font-size: 1.05rem;
    display: flex;
    justify-content: space-between;
  }
  .mat {
    color: var(--ink-dim);
    font-weight: 400;
  }
  ol {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    gap: 0.2rem;
  }
  li {
    display: grid;
    grid-template-columns: 1.6rem 1fr auto 1fr;
    gap: 0.5rem;
    align-items: baseline;
    padding: 0.35rem 0.4rem;
    border-radius: 6px;
    font-size: 0.95rem;
  }
  li.running {
    background: var(--panel-2);
    outline: 2px solid var(--ok);
  }
  li.complete {
    color: var(--ink-dim);
  }
  .n {
    color: var(--ink-dim);
    font-size: 0.8rem;
  }
  .red {
    color: var(--red-bright);
    text-align: right;
  }
  .blue {
    color: var(--blue-bright);
  }
  li.complete .red,
  li.complete .blue {
    color: var(--ink-dim);
  }
  .score {
    font-weight: 700;
    min-width: 3rem;
    text-align: center;
  }
  .empty {
    color: var(--ink-dim);
  }
</style>
