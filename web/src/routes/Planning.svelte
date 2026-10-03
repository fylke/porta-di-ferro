<script lang="ts">
  import { onMount } from 'svelte';
  import { api, type ForecastView, type ReportView, type Timings } from '../api';
  import { hall } from '../lib/event.svelte';
  import { clockOf, mmss } from '../lib/clock-of-day';
  import { t } from '../lib/i18n.svelte';

  /**
   * Planning the day (phase 4, #64 and #6). How long things take, when the day starts and
   * the venue closes, and -- before the entries are in -- how many each discipline expects:
   * enough for the mat board to lay the day out and say whether it fits. Once fencing has
   * started, the measured day: every match as the logs time it, the ones not to learn from
   * marked, and the averages written back into the timings, for today or for the next event.
   */
  let forecast = $state<ForecastView | null>(null);
  let report = $state<ReportView | null>(null);
  let error = $state('');
  let saved = $state('');
  let busy = $state(false);

  // The form, in minutes, seeded from the timings in force.
  let match = $state(4);
  let changeover = $state(1);
  let beforeElims = $state(15);
  let start = $state('09:00');
  let close = $state('');
  let expected = $state<Record<string, number>>({});
  let seeded = false;

  function seed(f: ForecastView) {
    const tm = f.timings;
    match = (tm.match ?? 240) / 60;
    changeover = (tm.changeover ?? 60) / 60;
    beforeElims = (tm.beforeElims ?? 900) / 60;
    start = tm.start ?? '09:00';
    close = tm.close ?? '';
    expected = { ...(f.expected ?? {}) };
  }

  async function load() {
    try {
      forecast = await api.forecast();
      if (!seeded) {
        seed(forecast);
        seeded = true;
      }
      if (forecast.live) report = await api.report();
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    }
  }

  onMount(() => {
    void load();
    void hall.refresh();
  });

  const disciplines = $derived((hall.view?.disciplines ?? []).filter((d) => !d.error));

  async function run(f: () => Promise<unknown>, done = '') {
    error = '';
    saved = '';
    busy = true;
    try {
      await f();
      saved = done;
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    } finally {
      busy = false;
    }
  }

  function saveTimings(e: SubmitEvent) {
    e.preventDefault();
    const tm: Timings = {
      match: Math.round(match * 60),
      changeover: Math.round(changeover * 60),
      beforeElims: Math.round(beforeElims * 60),
      start,
      close,
    };
    void run(async () => (forecast = await api.setTimings(tm)), t('Saved'));
  }

  function saveExpected(slug: string, n: number) {
    void run(async () => (forecast = await api.setExpected({ [slug]: Math.max(0, Math.round(n || 0)) })));
  }

  function setSession(slug: string, n: number) {
    void run(async () => (forecast = await api.setSessions({ [slug]: n })));
  }
  function setFinalsLast(last: boolean) {
    void run(async () => (forecast = await api.setFinalsLast(last)));
  }

  function learn() {
    void run(async () => {
      forecast = await api.learnTimings();
      seed(forecast);
    }, t('The timings are now the measured ones.'));
  }

  function keep() {
    void run(async () => {
      const res = await api.keepTimings();
      saved = t('Kept for new events, in {file}', { file: res.file });
    });
  }

  function mark(key: string, anomaly: boolean) {
    void run(async () => (report = await api.setAnomaly(key, anomaly)));
  }
</script>

<section>
  <h2>{t('Planning the day')}</h2>
  <p class="dim">
    {t('How long things take, and when the day starts and ends. The mat board times every card from these until the day has a pace of its own, and says whether it all fits.')}
  </p>

  <form class="fields" onsubmit={saveTimings}>
    <label>
      {t('A match, in minutes')}
      <input type="number" min="1" max="60" step="0.5" bind:value={match} />
    </label>
    <label>
      {t('Between matches')}
      <input type="number" min="0" max="30" step="0.5" bind:value={changeover} />
    </label>
    <label>
      {t('Before the eliminations')}
      <input type="number" min="0" max="240" step="5" bind:value={beforeElims} />
    </label>
    <label>
      {t('First match')}
      <input type="time" bind:value={start} />
    </label>
    <label>
      {t('The venue closes')}
      <input type="time" bind:value={close} />
    </label>
    <div class="actions">
      <button class="save" type="submit" disabled={busy}>{t('Save')}</button>
    </div>
  </form>
  <p class="dim small">{t('Breaks in the programme, such as lunch, stop the mats: give them a start and an end there.')}</p>

  {#if disciplines.length > 1}
    <!-- Which disciplines run side by side and which one after another (#136). -->
    <h3>{t('The order of the day')}</h3>
    <p class="dim small">
      {t('Disciplines in the same block run side by side; a block starts once every discipline of the blocks before it is done.')}
    </p>
    <ul class="expected">
      {#each disciplines as d (d.slug)}
        <li>
          <span class="strong">{d.name || t('Unnamed')}</span>
          <label>
            {t('Block')}
            <select value={forecast?.sessions?.[d.slug] ?? 1} disabled={busy} onchange={(e) => setSession(d.slug, Number(e.currentTarget.value))}>
              {#each disciplines as _, i (i)}<option value={i + 1}>{i + 1}</option>{/each}
            </select>
          </label>
        </li>
      {/each}
    </ul>
    <label class="check">
      <input type="checkbox" checked={forecast?.finalsLast ?? false} disabled={busy} onchange={(e) => setFinalsLast(e.currentTarget.checked)} />
      {t('Every final at the end of the day, one after another on mat 1')}
    </label>
  {/if}

  {#if disciplines.length > 0}
    <h3>{t('Before the entries are in')}</h3>
    <p class="dim small">{t('How many each discipline expects. Until that many are entered and the pools drawn, the board plans for this many.')}</p>
    <ul class="expected">
      {#each disciplines as d (d.slug)}
        <li>
          <span class="strong">{d.name || t('Unnamed')}</span>
          <span class="dim-inline">{t('{n} entered', { n: d.competitors })}</span>
          <label>
            {t('Expected')}
            <input
              type="number"
              min="0"
              max="200"
              value={expected[d.slug] ?? ''}
              onchange={(e) => {
                const n = Number(e.currentTarget.value);
                expected = { ...expected, [d.slug]: n };
                saveExpected(d.slug, n);
              }}
            />
          </label>
        </li>
      {/each}
    </ul>
  {/if}

  {#if forecast}
    <p class="summary">
      {#if forecast.end}
        {forecast.live ? t('Forecast to end at {time}', { time: clockOf(forecast.end) }) : t('Planned to end at {time}', { time: clockOf(forecast.end) })}
      {:else}
        {t('Nothing to plan yet: enter competitors, or say how many are expected.')}
      {/if}
    </p>
    {#if forecast.live}
      <ul class="pace">
        {#each forecast.pace as p (p.mat)}
          <li>
            {t('Mat {n}', { n: p.mat })}: {t('{time} a match', { time: mmss(p.match) })}
            <span class="dim-inline">{p.samples === 0 ? t('at the hall’s pace') : p.samples === 1 ? t('from 1 match') : t('from {n} matches', { n: p.samples })}</span>
          </li>
        {/each}
      </ul>
    {/if}
  {/if}

  {#if error}<p class="err">{error}</p>{/if}
  {#if saved}<p class="ok">{saved}</p>{/if}

  {#if report && report.matches.length > 0}
    <details>
      <summary>{t('The day so far, measured')} ({report.matches.length})</summary>
      <p class="dim small">
        {t('Mark a match whose time says nothing about the next event, such as a long injury break or a clock nobody stopped, and it is left out.')}
      </p>
      <p class="averages">
        {t('A match took {match} on average, and {changeover} between matches, from {n} matches.', {
          match: mmss(report.match),
          changeover: mmss(report.changeover),
          n: report.samples,
        })}
      </p>
      <div class="actions">
        <button disabled={busy || report.samples === 0} onclick={learn}>{t('Use these as the timings')}</button>
        <button disabled={busy} onclick={keep}>{t('Keep the timings for new events')}</button>
      </div>
      <div class="scroller">
        <table>
          <thead>
            <tr>
              <th>{t('Started')}</th>
              <th>{t('Match')}</th>
              <th>{t('Mat')}</th>
              <th>{t('Took')}</th>
              <th>{t('Gap')}</th>
              <th>{t('Leave out')}</th>
            </tr>
          </thead>
          <tbody>
            {#each report.matches as m (m.key)}
              <tr class:anomaly={m.anomaly}>
                <td class="mono">{clockOf(m.started)}</td>
                <td>
                  <span class="dim-inline first">{m.disciplineName}</span>
                  {m.red} {t('v')} {m.blue}
                </td>
                <td class="mono">{m.mat}</td>
                <td class="mono">{mmss(m.seconds)}</td>
                <td class="mono">{m.changeover ? mmss(m.changeover) : ''}</td>
                <td>
                  <input
                    type="checkbox"
                    checked={m.anomaly}
                    aria-label={t('Leave {match} out', { match: `${m.red} ${t('v')} ${m.blue}` })}
                    onchange={(e) => mark(m.key, e.currentTarget.checked)}
                  />
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    </details>
  {:else if forecast?.live === false}
    <div class="actions">
      <button disabled={busy} onclick={keep}>{t('Keep the timings for new events')}</button>
    </div>
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
  }
  h3 {
    margin: 1rem 0 0.3rem;
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
    margin: 0 0 0.7rem;
    max-width: 48rem;
  }
  .small {
    font-size: 0.82rem;
  }
  .dim-inline {
    color: var(--ink-dim);
    font-size: 0.85rem;
    margin-left: 0.4rem;
  }
  .dim-inline.first {
    display: block;
    margin-left: 0;
    font-size: 0.75rem;
  }
  .fields {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(9.5rem, 1fr));
    gap: 0.6rem;
    align-items: end;
    margin-bottom: 0.4rem;
  }
  .fields label,
  .expected label {
    display: grid;
    gap: 0.3rem;
    font-size: 0.85rem;
    color: var(--ink-dim);
  }
  .fields input,
  .expected input {
    padding: 0.4rem 0.5rem;
    min-width: 0;
  }
  .expected {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    gap: 0.4rem;
  }
  .expected li {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.3rem 0.8rem;
  }
  .expected label {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
    margin-left: auto;
  }
  .expected input {
    width: 5rem;
  }
  .strong {
    font-weight: 700;
  }
  label.check {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
    margin-top: 0.6rem;
    font-size: 0.9rem;
  }
  .summary {
    margin: 1rem 0 0.3rem;
    font-weight: 700;
  }
  .pace {
    margin: 0 0 0.6rem;
    padding-left: 1.1rem;
    font-size: 0.88rem;
    line-height: 1.5;
  }
  .actions {
    display: flex;
    flex-wrap: wrap;
    gap: 0.5rem;
    margin: 0.4rem 0;
  }
  .save {
    padding: 0.5rem 1rem;
    font-weight: 700;
    background: var(--ink);
    color: #0d0f14;
    border: none;
  }
  .err {
    color: var(--amber-bright);
  }
  .ok {
    color: var(--ok);
    font-size: 0.88rem;
    overflow-wrap: anywhere;
  }
  details {
    margin-top: 0.8rem;
  }
  summary {
    cursor: pointer;
    font-weight: 700;
  }
  .averages {
    margin: 0.4rem 0;
    font-size: 0.9rem;
  }
  .scroller {
    overflow-x: auto;
    max-height: 24rem;
    overflow-y: auto;
  }
  table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.85rem;
    min-width: 30rem;
  }
  th,
  td {
    padding: 0.3rem 0.4rem;
    text-align: left;
    border-bottom: 1px solid var(--line);
  }
  th {
    color: var(--ink-dim);
    font-weight: 500;
    font-size: 0.72rem;
  }
  tr.anomaly td {
    color: var(--ink-dim);
    text-decoration: line-through;
  }
  tr.anomaly td:last-child {
    text-decoration: none;
  }
</style>
