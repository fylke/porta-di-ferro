<script lang="ts">
  import { onMount } from 'svelte';
  import { Live, nameLookup } from './lib-display.svelte';
  import { t } from '../lib/i18n.svelte';
  import LangToggle from './LangToggle.svelte';

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

  type Paper = 'A4' | 'letter';
  let paper = $state<Paper>('A4');

  /**
   * Prints on a named paper size.
   *
   * The size lives in an `@page` rule, which a component stylesheet cannot reach and a
   * class cannot switch, so the rule is written into the document whole. Two buttons
   * rather than a line of text telling the organizer to go and check the printer: the
   * sheets are printed once, days before the event, and the only thing anybody wants at
   * that moment is the right paper coming out.
   */
  function printOn(size: Paper) {
    paper = size;
    const id = 'porta-page-size';
    let rule = document.getElementById(id) as HTMLStyleElement | null;
    if (!rule) {
      rule = document.createElement('style');
      rule.id = id;
      document.head.appendChild(rule);
    }
    rule.textContent = `@page { size: ${size}; margin: 12mm; }`;
    window.print();
  }

  const name = $derived(nameLookup(live.snapshot));
  const club = $derived.by(() => {
    const byId = new Map((live.snapshot?.competitors ?? []).map((c) => [c.id, c.club]));
    return (id: string) => byId.get(id) ?? '';
  });
</script>

<div class="sheets">
  <p class="noprint">
    <button class:chosen={paper === 'A4'} onclick={() => printOn('A4')}>{t('Print A4')}</button>
    <button class:chosen={paper === 'letter'} onclick={() => printOn('letter')}>{t('Print letter')}</button>
    {t('One sheet per pool, plus instructions. Turn background graphics off in the print dialog.')}
    <LangToggle compact />
  </p>

  {#each live.snapshot?.pools ?? [] as pool (pool.number)}
    <article>
      <header>
        <h1>{t('Pool {n}', { n: pool.number })}</h1>
        <span>{t('Mat {n}', { n: pool.mat })} &middot; {t('8 points or 3 minutes · differential scoring')}</span>
      </header>

      <table class="people">
        <thead>
          <tr><th>#</th><th>{t('Competitor')}</th><th>{t('Club')}</th></tr>
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
            <th class="red">{t('Red')}</th>
            <th class="s">{t('Score')}</th>
            <th class="w">{t('Warnings')}</th>
            <th class="blue group">{t('Blue')}</th>
            <th class="s">{t('Score')}</th>
            <th class="w">{t('Warnings')}</th>
          </tr>
        </thead>
        <tbody>
          {#each pool.matches as m (m.id)}
            <tr>
              <td>{m.order}</td>
              <td class="red">{name(m.red)}</td>
              <td class="s"></td>
              <td class="w"></td>
              <td class="blue group">{name(m.blue)}</td>
              <td class="s"></td>
              <td class="w"></td>
            </tr>
          {/each}
        </tbody>
      </table>

      <footer>{t('Match points: win 9, draw 6, loss 3. A penalty loss or a forfeit is recorded 0–8 and earns none. Second warning deducts a point; third loses the match.')}</footer>
    </article>
  {/each}

  <article class="instructions">
    <header>
      <h1>{t('How to calculate the eliminations')}</h1>
    </header>
    <p class="intro">
      {t('Use this page with the pool sheets. Finish every pool match first. Rank all eligible competitors together, not separately by pool.')}
    </p>

    <section>
      <h2>{t('1. Record each match')}</h2>
      <p>{t('Record both final scores and each competitor’s result. Match points are: win 9, draw 6, loss 3. A penalty loss or forfeit earns 0 match points.')}</p>
    </section>

    <section>
      <h2>{t('2. Calculate each competitor’s indices')}</h2>
      <p>{t('Total each competitor’s match points, wins, points scored, and points conceded. Divide by their counted completed pool matches; ignore matches voided because someone withdrew.')}</p>
      <dl class="indices">
        <div><dt>{t('Match point index (MPI)')}</dt><dd>{t('Total match points ÷ completed matches')}</dd></div>
        <div><dt>{t('Victory index (VI)')}</dt><dd>{t('Wins ÷ completed matches')}</dd></div>
        <div><dt>{t('Score index (SI)')}</dt><dd>{t('(Points scored − points conceded) ÷ completed matches')}</dd></div>
        <div><dt>{t('Reception index (RI)')}</dt><dd>{t('Points conceded ÷ completed matches; lower is better')}</dd></div>
      </dl>
    </section>

    <section>
      <h2>{t('3. Rank everyone across all pools')}</h2>
      <p>{t('Compare indices in this order: highest MPI, highest VI, highest SI, then lowest RI.')}</p>
      <p>{t('If still tied, use the result between them if they fenced each other. For a paper-only event, draw lots for any remaining tie and record the result. The app resolves remaining ties with its repeatable seeded draw when results are entered.')}</p>
    </section>

    <section>
      <h2>{t('4. Choose who qualifies')}</h2>
      <p>{t('Count only competitors who have not withdrawn.')}</p>
      <ul>
        <li>{t('8 or more competitors: ranks 1–8 qualify.')}</li>
        <li>{t('4–7 competitors: ranks 1–4 qualify.')}</li>
        <li>{t('2–3 competitors: ranks 1–2 qualify.')}</li>
        <li>{t('Fewer than 2 competitors: no eliminations.')}</li>
      </ul>
    </section>

    <section>
      <h2>{t('5. Seed the bracket')}</h2>
      <p>{t('8 qualifiers: 1 vs 8, 4 vs 5, 2 vs 7, and 3 vs 6. Winners of the first two matches meet; winners of the last two meet.')}</p>
      <p>{t('4 qualifiers: 1 vs 4 and 2 vs 3. 2 qualifiers: 1 vs 2. The higher seed is red. With 4 or 8 qualifiers, the semi-final losers fence for bronze. Eliminations are single-elimination; use sudden death to decide a tied match.')}</p>
    </section>
  </article>

  {#if (live.snapshot?.pools ?? []).length === 0}
    <p class="noprint">{t('Pools have not been drawn yet.')}</p>
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
  /* Which size was printed last, so a second run of the same sheets is one click and not
     a guess. */
  .noprint button.chosen {
    background: #111;
    color: #fff;
    border-color: #111;
  }
  article {
    max-width: 46rem;
    margin: 0 auto 2rem;
    page-break-after: always;
    break-after: page;
  }
  .instructions {
    page-break-after: auto;
    break-after: auto;
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
     survive a black-and-white printer. Left and right are deliberately not printed: the
     score keeper's screen fixes red to the left, but which side of the mat someone starts
     on is the head referee's business, and a sheet that claimed otherwise would be wrong
     as often as it was right. */
  .red {
    font-weight: 700;
  }
  .blue {
    font-weight: 700;
  }
  .s {
    width: 3rem;
    text-align: center;
  }
  /* One warning box per fencer, not one per bout. Warnings are counted per competitor and
     the count is what decides the match -- the second costs a point and the third ends it
     -- so a shared box cannot record the thing the paper exists to record. */
  .w {
    width: 4.5rem;
  }
  /* Where one fencer's columns end and the other's begin, so the two score-and-warnings
     pairs cannot be read as one run of four boxes. */
  .group {
    border-left: 2px solid #111;
  }
  footer {
    font-size: 0.78rem;
    color: #444;
    line-height: 1.5;
  }
  .instructions header {
    margin-bottom: 0.6rem;
  }
  .instructions .intro,
  .instructions section p,
  .instructions li {
    font-size: 0.82rem;
    line-height: 1.35;
  }
  .instructions .intro {
    margin: 0 0 0.6rem;
  }
  .instructions section {
    margin: 0 0 0.55rem;
    break-inside: avoid;
  }
  .instructions h2 {
    margin: 0 0 0.2rem;
    font-size: 0.95rem;
  }
  .instructions section p {
    margin: 0 0 0.3rem;
  }
  .indices {
    display: grid;
    grid-template-columns: 1fr 1.45fr;
    gap: 0.15rem 1rem;
    margin: 0;
    font-size: 0.78rem;
  }
  .indices div {
    display: contents;
  }
  .indices dt {
    font-weight: 700;
  }
  .indices dd {
    margin: 0;
  }
  .instructions ul {
    margin: 0.2rem 0 0;
    padding-left: 1.3rem;
    columns: 2;
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
    .instructions section {
      break-inside: avoid;
    }
  }
</style>
