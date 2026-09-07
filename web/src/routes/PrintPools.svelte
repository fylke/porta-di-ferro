<script lang="ts">
  import { onMount } from 'svelte';
  import { Live, nameLookup } from './lib-display.svelte';

  /**
   * Printable pool sheets: the paper fallback, and what tier 2 and tier 3 of the fallback
   * ladder both rest on. One sheet per pool, every match with its assigned colours, and
   * room to write scores and warnings by hand (design §6 item 11).
   *
   * Print these before the event, not on the day.
   */
  const live = new Live();
  onMount(() => {
    live.start();
    return () => live.stop();
  });

  const name = $derived(nameLookup(live.snapshot));
  const club = $derived.by(() => {
    const byId = new Map((live.snapshot?.competitors ?? []).map((c) => [c.id, c.club]));
    return (id: string) => byId.get(id) ?? '';
  });
</script>

<div class="sheets">
  <p class="noprint">
    <button onclick={() => window.print()}>Print</button>
    One sheet per pool. Check the printer is on A4 and that backgrounds are off.
  </p>

  {#each live.snapshot?.pools ?? [] as pool (pool.number)}
    <article>
      <header>
        <h1>Pool {pool.number}</h1>
        <span>Mat {pool.mat} &middot; 8 points or 3 minutes &middot; differential scoring</span>
      </header>

      <table class="people">
        <thead>
          <tr><th>#</th><th>Competitor</th><th>Club</th></tr>
        </thead>
        <tbody>
          {#each pool.competitors as id, i (id)}
            <tr><td>{i + 1}</td><td>{name(id)}</td><td>{club(id)}</td></tr>
          {/each}
        </tbody>
      </table>

      <table class="matches">
        <thead>
          <tr>
            <th>#</th>
            <th class="red">Red</th>
            <th class="s">Score</th>
            <th class="s">Score</th>
            <th class="blue">Blue</th>
            <th class="w">Warnings</th>
          </tr>
        </thead>
        <tbody>
          {#each pool.matches as m (m.id)}
            <tr>
              <td>{m.order}</td>
              <td class="red">{name(m.red)}</td>
              <td class="s"></td>
              <td class="s"></td>
              <td class="blue">{name(m.blue)}</td>
              <td class="w"></td>
            </tr>
          {/each}
        </tbody>
      </table>

      <footer>
        Match points: win 9, draw 6, loss 3. A penalty loss or a forfeit is recorded 0&ndash;8
        and earns none. Second warning deducts a point; third loses the match.
      </footer>
    </article>
  {/each}

  {#if (live.snapshot?.pools ?? []).length === 0}
    <p class="noprint">Pools have not been drawn yet.</p>
  {/if}
</div>

<style>
  .sheets {
    background: #fff;
    color: #111;
    min-height: 100dvh;
    padding: 1rem;
  }
  .noprint {
    display: flex;
    gap: 1rem;
    align-items: center;
    font-family: var(--font);
    color: #444;
  }
  .noprint button {
    padding: 0.5rem 1.2rem;
    font-weight: 700;
    background: #eee;
    border: 1px solid #bbb;
    color: #111;
  }
  article {
    max-width: 46rem;
    margin: 0 auto 2rem;
    page-break-after: always;
    break-after: page;
  }
  header {
    display: flex;
    justify-content: space-between;
    align-items: baseline;
    border-bottom: 2px solid #111;
    padding-bottom: 0.4rem;
    margin-bottom: 1rem;
  }
  h1 {
    margin: 0;
    font-size: 1.6rem;
  }
  header span {
    font-size: 0.85rem;
    color: #444;
  }
  table {
    width: 100%;
    border-collapse: collapse;
    margin-bottom: 1.2rem;
    font-size: 0.9rem;
  }
  th,
  td {
    border: 1px solid #999;
    padding: 0.35rem 0.5rem;
    text-align: left;
  }
  th {
    background: #f0f0f0;
    font-size: 0.75rem;
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }
  .matches td {
    height: 2rem;
  }
  /* The colours are printed as words rather than fills, because a pool sheet has to
     survive a black-and-white printer. */
  .red {
    font-weight: 700;
  }
  .blue {
    font-weight: 700;
  }
  th.red::after {
    content: ' (left)';
    font-weight: 400;
    text-transform: none;
  }
  th.blue::after {
    content: ' (right)';
    font-weight: 400;
    text-transform: none;
  }
  .s {
    width: 3.5rem;
    text-align: center;
  }
  .w {
    width: 7rem;
  }
  footer {
    font-size: 0.78rem;
    color: #444;
    line-height: 1.5;
  }
  @media print {
    .noprint {
      display: none;
    }
    .sheets {
      padding: 0;
    }
    article {
      margin-bottom: 0;
    }
  }
</style>
