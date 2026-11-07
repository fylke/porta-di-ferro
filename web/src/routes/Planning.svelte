<script lang="ts">
  import { onMount } from 'svelte';
  import { api, apiIn, type Break, type ForecastView, type ReportView, type Timings } from '../api';
  import { hall } from '../lib/event.svelte';
  import { tour } from '../lib/tour.svelte';
  import { clockOf, mmss } from '../lib/clock-of-day';
  import { t } from '../lib/i18n.svelte';
  import Programme from './Programme.svelte';
  import SaveButton from './SaveButton.svelte';
  import TourExample from './TourExample.svelte';
  import { errorText } from '../lib/errors';

  /**
   * Planning the day (phase 4, #64, #6, #149). How long things take, when the day starts
   * and the venue closes, the mats, the hall's breaks, and for each discipline how many it
   * expects, how many mats its pools use, its block of the day, and its gear check and staff
   * briefing. The programme is generated from all of it and shown at the end, with what a
   * day usually has and this one lacks pointed out. Once fencing has started, the measured
   * day: every match as the logs time it, the ones not to learn from marked, and the
   * averages written back into the timings, for today or for the next event.
   */
  let forecast = $state<ForecastView | null>(null);
  let report = $state<ReportView | null>(null);
  let error = $state('');
  let note = $state('');
  let busy = $state(false);

  // The form, in minutes, seeded from the timings in force.
  let match = $state(4);
  let changeover = $state(1);
  let beforeElims = $state(15);
  let start = $state('09:00');
  let close = $state('');
  let expected = $state<Record<string, number>>({});
  let timingsDirty = $state(false);
  let seeded = false;

  // The hall's breaks, as edited here before saving (#149).
  let breaks = $state<Break[]>([]);
  let breaksDirty = $state(false);

  function seed(f: ForecastView) {
    const tm = f.timings;
    match = (tm.match ?? 240) / 60;
    changeover = (tm.changeover ?? 60) / 60;
    beforeElims = (tm.beforeElims ?? 900) / 60;
    start = tm.start ?? '09:00';
    close = tm.close ?? '';
    expected = { ...(f.expected ?? {}) };
    breaks = (f.breaks ?? []).map((b) => ({ ...b }));
    timingsDirty = breaksDirty = false;
  }

  // Each mat's location, as edited here (#123, #149). A time a mat is away is still kept
  // and shown where one was set, but lunch is a break for the whole hall now.
  let matEdits = $state<{ mat: number; name: string; away: { from: string; to: string }[] }[]>([]);
  async function loadMats() {
    try {
      const view = await api.mats();
      matEdits = view.mats.map((m) => ({ mat: m.mat, name: m.name ?? '', away: (m.away ?? []).map((a) => ({ ...a })) }));
    } catch {
      // The rest of the panel works without it.
    }
  }
  function saveMat(i: number) {
    const m = matEdits[i];
    void run(async () => {
      await api.setMat(m.mat, { name: m.name, away: m.away.filter((a) => a.from && a.to) });
      forecast = await api.forecast();
    });
  }
  function setMatCount(n: number) {
    void run(async () => {
      await api.setMats(n);
      await loadMats();
      forecast = await api.forecast();
    });
  }

  async function load() {
    void loadMats();
    try {
      forecast = await api.forecast();
      if (!seeded) {
        seed(forecast);
        seeded = true;
      }
      if (forecast.live) report = await api.report();
    } catch (e) {
      error = errorText(e);
    }
  }

  onMount(() => {
    void load();
    void hall.refresh();
  });

  // The disciplines' setup changes what there is to plan: look again when one changes.
  const shape = $derived(
    (hall.view?.disciplines ?? []).map((d) => `${d.slug}:${d.setUp}:${d.competitors}:${d.pools}`).join('|'),
  );
  $effect(() => {
    void shape;
    if (seeded) void api.forecast().then((f) => (forecast = f)).catch(() => {});
  });

  const disciplines = $derived((hall.view?.disciplines ?? []).filter((d) => !d.error));
  // The tour, on an event with nothing in it yet: what these panels look like with something.
  const example = $derived(tour.active && disciplines.length === 0);

  async function run(f: () => Promise<unknown>, done = '') {
    error = '';
    note = '';
    busy = true;
    try {
      await f();
      note = done;
      return true;
    } catch (e) {
      error = errorText(e);
      return false;
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
    void run(async () => {
      forecast = await api.setTimings(tm);
      timingsDirty = false;
    });
  }

  function addBreak() {
    breaks = [...breaks, breaks.length === 0 ? { label: '', from: '12:00', to: '12:45' } : { label: '', from: '', to: '' }];
    breaksDirty = true;
  }
  function removeBreak(i: number) {
    breaks = breaks.filter((_, k) => k !== i);
    breaksDirty = true;
  }
  function saveBreaks() {
    void run(async () => {
      forecast = await api.setBreaks(breaks);
      breaks = (forecast.breaks ?? []).map((b) => ({ ...b }));
      breaksDirty = false;
    });
  }

  /** The lunch break, for the box that says whether the day has one (#153). */
  function isLunch(b: Break): boolean {
    const label = (b.label ?? '').trim().toLocaleLowerCase();
    return label === '' || label === 'lunch' || label === t('Lunch').toLocaleLowerCase();
  }
  const hasLunch = $derived(breaks.some(isLunch));
  function setLunch(on: boolean) {
    breaks = on ? [...breaks, { label: '', from: '12:00', to: '12:45' }] : breaks.filter((b) => !isLunch(b));
    saveBreaks();
  }

  /** How many mats a discipline's pools use: decided here, once its numbers are known (#153). */
  function setDisciplineMats(slug: string, mats: number) {
    const now = forecast?.mats?.[slug];
    if (!now) return;
    void run(async () => {
      await apiIn(slug).saveTournament(mats, now.minPoolSize, now.maxPoolSize);
      forecast = await api.forecast();
    });
  }

  function saveExpected(slug: string, n: number) {
    void run(async () => (forecast = await api.setExpected({ [slug]: Math.max(0, Math.round(n || 0)) })));
  }
  function setPrep(slug: string, field: 'gearCheck' | 'briefing', minutes: number) {
    const now = forecast?.prep?.[slug] ?? {};
    void run(async () => (forecast = await api.setPrep({ [slug]: { ...now, [field]: Math.max(0, Math.round((minutes || 0) * 60)) } })));
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
      note = t('New events will start from these timings. They are kept in {file}', { file: res.file });
    });
  }

  function mark(key: string, anomaly: boolean) {
    void run(async () => (report = await api.setAnomaly(key, anomaly)));
  }

  function gapText(g: ForecastView['gaps'][number]): string {
    switch (g.kind) {
      case 'lunch':
        return t('The day has no lunch break.');
      case 'gear-check':
        return t('{name} has no gear check.', { name: g.name || t('Unnamed') });
      case 'briefing':
        return t('{name} has no staff briefing.', { name: g.name || t('Unnamed') });
    }
  }
</script>

<section>
  <h2>{t('Planning the day')}</h2>
  <p class="dim">
    {t('How long things take, and when the day starts and ends. All times are in minutes. The mat board times every card from these until the day has a pace of its own, and says whether it all fits.')}
  </p>

  <div data-tour="plan-timings">
  <form class="fields" onsubmit={saveTimings} oninput={() => (timingsDirty = true)}>
    <label>
      <span class="tip" title={t('From the time that the competitors step on the mat, and until they step off it.')}>{t('One match takes')}</span>
      <input type="number" min="1" max="60" step="0.5" bind:value={match} />
    </label>
    <label>
      {t('Between matches')}
      <input type="number" min="0" max="30" step="0.5" bind:value={changeover} />
    </label>
    <label>
      {t('Break before eliminations')}
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
    <div class="apply">
      <SaveButton type="submit" dirty={timingsDirty} {busy} label={t('Apply')} done={t('Applied')} />
    </div>
  </form>

  <!-- How many mats, and where each one is (#123, #149). -->
  <h3>{t('The mats')}</h3>
  <div class="count">
    <button class="secondary" aria-label={t('One mat fewer')} disabled={busy || matEdits.length <= 1} onclick={() => setMatCount(matEdits.length - 1)}>&minus;</button>
    <span class="strong">{matEdits.length === 1 ? t('1 mat') : t('{n} mats', { n: matEdits.length })}</span>
    <button class="secondary" aria-label={t('One mat more')} disabled={busy || matEdits.length >= 8} onclick={() => setMatCount(matEdits.length + 1)}>+</button>
    {#if forecast?.matsRecommended}
      <span class="hint">{t('Recommended for what the disciplines expect: {n}', { n: forecast.matsRecommended })}</span>
    {/if}
  </div>
  <ul class="mats">
    {#each matEdits as m, i (m.mat)}
      <li>
        <span class="strong">{t('Mat {n}', { n: m.mat })}</span>
        <input
          class="mat-name"
          bind:value={m.name}
          placeholder={t('Location (optional)')}
          aria-label={t('Location of mat {n}', { n: m.mat })}
          onchange={() => saveMat(i)}
        />
        {#each m.away as a, j (j)}
          <span class="away">
            {t('away')} {a.from}&ndash;{a.to}
            <button class="secondary small" aria-label={t('Remove')} onclick={() => ((m.away = m.away.filter((_, k) => k !== j)), saveMat(i))}>&times;</button>
          </span>
        {/each}
      </li>
    {/each}
  </ul>

  <!-- The hall's breaks: lunch, the prize giving (#149). -->
  <h3>{t('Breaks')}</h3>
  <p class="dim small">{t('No mat fences during a break, and it goes in the programme. Most days have a lunch.')}</p>
  <ul class="breaks">
    {#each breaks as b, i (i)}
      <li>
        <input class="label" bind:value={b.label} oninput={() => (breaksDirty = true)} placeholder={t('Lunch')} aria-label={t('What the break is')} />
        <input type="time" bind:value={b.from} oninput={() => (breaksDirty = true)} aria-label={t('From')} />
        &ndash;
        <input type="time" bind:value={b.to} oninput={() => (breaksDirty = true)} aria-label={t('Until')} />
        <button class="secondary small" aria-label={t('Remove')} onclick={() => removeBreak(i)}>&times;</button>
      </li>
    {/each}
  </ul>
  {#if example && breaks.length === 0}<TourExample kind="breaks" />{/if}
  <div class="actions">
    <button class="secondary" onclick={addBreak}>{t('Add a break')}</button>
    {#if breaksDirty || breaks.length > 0}<SaveButton dirty={breaksDirty} {busy} label={t('Apply')} done={t('Applied')} onclick={saveBreaks} />{/if}
  </div>

  {#if example}
    <h3>{t('Each discipline')}</h3>
    <TourExample kind="each" />
  {/if}

  {#if disciplines.length > 0}
    <!-- Each discipline: how many it expects, its block of the day, and what comes before
         its first pool (#64, #136, #149). -->
    <h3>{t('Each discipline')}</h3>
    <p class="dim small">
      {t('Until that many are entered and the pools drawn, the day is planned for the number expected. By default the staff briefing ends 30 minutes before a discipline starts, and the gear check 15 minutes before. Mats are recommended at one for each pool, up to four.')}
      {#if disciplines.length > 1}{t('Disciplines in the same block run side by side; a block starts once every discipline of the blocks before it is done.')}{/if}
    </p>
    <div class="scroller">
      <table class="each">
        <thead>
          <tr>
            <th>{t('Discipline')}</th>
            <th>{t('Expected')}</th>
            <th>{t('Mats')}</th>
            {#if disciplines.length > 1}<th>{t('Block')}</th>{/if}
            <th>{t('Gear check')}</th>
            <th>{t('Staff briefing')}</th>
          </tr>
        </thead>
        <tbody>
          {#each disciplines as d (d.slug)}
            <tr>
              <td>
                <span class="strong">{d.name || t('Unnamed')}</span>
                <span class="dim-inline">{t('{n} entered', { n: d.competitors })}</span>
              </td>
              <td>
                <input
                  type="number"
                  min="0"
                  max="200"
                  aria-label={t('Expected in {name}', { name: d.name })}
                  value={expected[d.slug] ?? ''}
                  onchange={(e) => {
                    const n = Number(e.currentTarget.value);
                    expected = { ...expected, [d.slug]: n };
                    saveExpected(d.slug, n);
                  }}
                />
              </td>
              <td class="nowrap">
                {#if forecast?.mats?.[d.slug]}
                  {@const advice = forecast.mats[d.slug]}
                  <select
                    aria-label={t('Mats of {name}', { name: d.name })}
                    value={advice.mats}
                    disabled={busy || (advice.pools ?? 0) > 0}
                    title={(advice.pools ?? 0) > 0 ? t('Its pools are drawn: change this on its admin page, and draw again.') : ''}
                    onchange={(e) => setDisciplineMats(d.slug, Number(e.currentTarget.value))}
                  >
                    {#each [1, 2, 3, 4] as n (n)}<option value={n}>{n}</option>{/each}
                  </select>
                  {#if advice.recommended}
                    <span class="dim-inline" class:differs={advice.recommended !== advice.mats}>
                      {t('{n} recommended', { n: advice.recommended })}
                    </span>
                  {/if}
                {/if}
              </td>
              {#if disciplines.length > 1}
                <td>
                  <select
                    aria-label={t('Block of {name}', { name: d.name })}
                    value={forecast?.sessions?.[d.slug] ?? 1}
                    disabled={busy}
                    onchange={(e) => setSession(d.slug, Number(e.currentTarget.value))}
                  >
                    {#each disciplines as _, i (i)}<option value={i + 1}>{i + 1}</option>{/each}
                  </select>
                </td>
              {/if}
              <td>
                <input
                  type="number"
                  min="0"
                  max="240"
                  step="5"
                  aria-label={t('Gear check of {name}, in minutes', { name: d.name })}
                  value={(forecast?.prep?.[d.slug]?.gearCheck ?? 0) / 60 || ''}
                  onchange={(e) => setPrep(d.slug, 'gearCheck', Number(e.currentTarget.value))}
                />
              </td>
              <td>
                <input
                  type="number"
                  min="0"
                  max="240"
                  step="5"
                  aria-label={t('Staff briefing of {name}, in minutes', { name: d.name })}
                  value={(forecast?.prep?.[d.slug]?.briefing ?? 0) / 60 || ''}
                  onchange={(e) => setPrep(d.slug, 'briefing', Number(e.currentTarget.value))}
                />
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
    <div class="checks">
      {#if disciplines.length > 1}
        <label class="check">
          <input type="checkbox" checked={forecast?.finalsLast ?? false} disabled={busy} onchange={(e) => setFinalsLast(e.currentTarget.checked)} />
          {t('Run the finals separately, at the end of the day')}
        </label>
      {/if}
      <!-- A day pointed out as having no lunch has to be able to get one, here (#153). -->
      <label class="check">
        <input type="checkbox" checked={hasLunch} disabled={busy} onchange={(e) => setLunch(e.currentTarget.checked)} />
        {t('Break for lunch')}
      </label>
    </div>
  {/if}
  </div>

  {#if error}<p class="err">{error}</p>{/if}

  {#if forecast}
    <!-- The programme, generated from all of the above (#149). -->
    <div class="programme" data-tour="plan-programme">
      <h3>{t('The programme')}</h3>
      {#if example}<TourExample kind="programme" />{/if}
      {#if forecast.end}
        <p class="summary">
          {forecast.live ? t('Forecast to end at {time}', { time: clockOf(forecast.end) }) : t('Estimated to end at {time}', { time: clockOf(forecast.end) })}
        </p>
      {/if}
      {#each forecast.gaps ?? [] as g, i (i)}
        <p class="warn">{gapText(g)}</p>
      {/each}
      {#if !example}
        <Programme rows={forecast.programme} empty={t('Nothing to plan yet: add a discipline, and say how many it expects.')} />
      {/if}
    </div>
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
        <!-- Only here (#153): what is worth carrying to the next event is what a day measured,
             not numbers somebody has just typed. -->
        <button class="secondary" disabled={busy} onclick={keep}>{t('Remember for new events')}</button>
      </div>
      {#if note}<p class="note">{note}</p>{/if}
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
    margin: 1.2rem 0 0.4rem;
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
  .hint {
    color: var(--ink-dim);
    font-size: 0.78rem;
    line-height: 1.4;
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
    grid-template-columns: repeat(auto-fit, minmax(9rem, 1fr));
    gap: 0.6rem;
    align-items: end;
  }
  .fields label {
    display: grid;
    gap: 0.3rem;
    font-size: 0.85rem;
    color: var(--ink-dim);
  }
  /* Says more when pointed at (#153). */
  .tip {
    width: fit-content;
    cursor: help;
    text-decoration: underline dotted;
    text-underline-offset: 0.2em;
  }
  .apply :global(button) {
    width: 100%;
  }
  .differs {
    color: var(--amber-bright);
  }
  .nowrap {
    white-space: nowrap;
  }
  .fields input {
    box-sizing: border-box;
    height: 2.3rem;
    padding: 0.4rem 0.5rem;
    min-width: 0;
  }
  .strong {
    font-weight: 700;
  }
  .count {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.6rem;
    margin-bottom: 0.5rem;
  }
  .mats,
  .breaks {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    gap: 0.45rem;
  }
  .mats li,
  .breaks li {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.4rem 0.7rem;
    font-size: 0.9rem;
  }
  .mat-name,
  .breaks .label {
    padding: 0.35rem 0.5rem;
    width: 14rem;
    max-width: 100%;
  }
  .breaks input[type='time'] {
    padding: 0.3rem 0.4rem;
  }
  .away {
    display: inline-flex;
    align-items: center;
    gap: 0.3rem;
    color: var(--ink-dim);
  }
  .small:is(button) {
    min-height: 2rem;
    padding: 0.2rem 0.55rem;
  }
  .scroller {
    overflow-x: auto;
  }
  table.each {
    border-collapse: collapse;
    font-size: 0.88rem;
    min-width: 30rem;
  }
  table.each td,
  table.each th {
    padding: 0.3rem 0.5rem 0.3rem 0;
    text-align: left;
    vertical-align: middle;
  }
  table.each input {
    width: 5rem;
    padding: 0.3rem 0.4rem;
  }
  .checks {
    display: flex;
    flex-wrap: wrap;
    gap: 0.4rem 1.5rem;
    margin-top: 0.7rem;
  }
  label.check {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
    font-size: 0.9rem;
  }
  .programme {
    margin-top: 0.4rem;
  }
  .summary {
    margin: 0.2rem 0 0.4rem;
    font-weight: 700;
  }
  .warn {
    margin: 0.2rem 0;
    color: var(--amber-bright);
    font-size: 0.9rem;
  }
  .pace {
    margin: 0.6rem 0;
    padding-left: 1.1rem;
    font-size: 0.88rem;
    line-height: 1.5;
  }
  .actions {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.5rem;
    margin: 0.5rem 0;
  }
  .err {
    color: var(--amber-bright);
  }
  .note {
    color: var(--ink-dim);
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
  .scroller:has(table:not(.each)) {
    max-height: 24rem;
    overflow-y: auto;
  }
  table:not(.each) {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.85rem;
    min-width: 30rem;
  }
  table:not(.each) th,
  table:not(.each) td {
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
