<script lang="ts">
  import { untrack } from 'svelte';
  import { api, type EventInfo, type ScheduleItem, type Snapshot } from '../api';
  import { t } from '../lib/i18n.svelte';

  /**
   * Where the welcome message, the day's agenda and the venue wifi are written
   * (issue #98). Admin only: everything here comes out on the info sheet on the door and
   * on every phone in the hall.
   *
   * Saved on a button rather than as you type. The other panels on this page save on
   * change because they hold one value each; this holds a paragraph and a list, and an
   * autosave that fires mid-sentence would push half-written text to every screen in the
   * venue.
   */
  let { snapshot, onchange }: { snapshot: Snapshot; onchange: () => void } = $props();

  // Seeded once, like the tournament setup above it: the organizer is the only writer,
  // and a field that rewrites itself under the cursor is worse than one briefly stale.
  const initial = untrack(() => snapshot.tournament.event ?? {});
  let welcome = $state(initial.welcome ?? '');
  let ssid = $state(initial.wifi?.ssid ?? '');
  let password = $state(initial.wifi?.password ?? '');
  let security = $state(initial.wifi?.security ?? 'WPA');
  let schedule = $state<ScheduleItem[]>(
    (initial.schedule ?? []).map((i) => ({ ...i })),
  );

  let saving = $state(false);
  let error = $state('');
  let saved = $state(false);

  function addRow(kind: ScheduleItem['kind'] = '') {
    schedule = [...schedule, { at: '', label: '', kind }];
    saved = false;
  }
  function removeRow(i: number) {
    schedule = schedule.filter((_, k) => k !== i);
    saved = false;
  }
  function move(i: number, by: -1 | 1) {
    const j = i + by;
    if (j < 0 || j >= schedule.length) return;
    const next = [...schedule];
    [next[i], next[j]] = [next[j], next[i]];
    schedule = next;
    saved = false;
  }
  function touch() {
    saved = false;
  }

  async function save() {
    error = '';
    saving = true;
    try {
      const event: EventInfo = {
        welcome,
        // Blank labels are how a row is removed; the server drops them too.
        schedule: schedule.filter((i) => (i.label ?? '').trim() !== ''),
        wifi: { ssid, password, security: security as 'WPA' | 'WEP' | 'nopass' },
      };
      await api.saveEvent(event);
      saved = true;
      onchange();
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    } finally {
      saving = false;
    }
  }
</script>

<section>
  <h2>{t('The day')}</h2>
  <p class="dim">
    {t('What goes on the info sheet by the door and on everyone’s phone. None of it touches a result.')}
    <a href="/info" target="_blank" rel="noreferrer">{t('See the info sheet')}</a>
  </p>

  <label class="block">
    <span class="lab">{t('Welcome message')}</span>
    <textarea
      rows="4"
      bind:value={welcome}
      oninput={touch}
      placeholder={t('Welcome to Stångebroslaget. Gear check from 08:30, first pools at 09:30.')}
    ></textarea>
  </label>

  <div class="sched">
    <span class="lab">{t('Programme')}</span>
    {#each schedule as row, i (i)}
      <div class="row">
        <input class="at" bind:value={row.at} oninput={touch} placeholder={t('09:00')} aria-label={t('Time')} />
        <input class="what" bind:value={row.label} oninput={touch} placeholder={t('Gear check')} aria-label={t('What happens')} />
        <select bind:value={row.kind} onchange={touch} aria-label={t('Kind')}>
          <option value="">{t('item')}</option>
          <option value="discipline">{t('discipline')}</option>
          <option value="break">{t('break')}</option>
        </select>
        <span class="rowbtns">
          <button type="button" onclick={() => move(i, -1)} aria-label={t('Move up')} title={t('Earlier')}>&uarr;</button>
          <button type="button" onclick={() => move(i, 1)} aria-label={t('Move down')} title={t('Later')}>&darr;</button>
          <button type="button" onclick={() => removeRow(i)} aria-label={t('Delete')} title={t('Delete this event')}>&times;</button>
        </span>
      </div>
    {/each}
    <p class="add">
      <button type="button" onclick={() => addRow('')}>{t('Add a row')}</button>
      <button type="button" onclick={() => addRow('discipline')}>{t('Add a discipline')}</button>
      <button type="button" onclick={() => addRow('break')}>{t('Add a break')}</button>
    </p>
  </div>

  <div class="wifi">
    <span class="lab">{t('Venue wifi')}</span>
    <p class="dim small">{t('Printed on the info sheet as a code to scan and as text to type. It is the guest network password, kept in your own tournament file.')}</p>
    <div class="fields">
      <label>
        {t('Network name')}
        <input bind:value={ssid} oninput={touch} placeholder={t('Hall-Guest')} />
      </label>
      <label>
        {t('Password')}
        <input bind:value={password} oninput={touch} disabled={security === 'nopass'} />
      </label>
      <label>
        {t('Security')}
        <select bind:value={security} onchange={touch}>
          <option value="WPA">WPA/WPA2</option>
          <option value="WEP">WEP</option>
          <option value="nopass">{t('open')}</option>
        </select>
      </label>
    </div>
  </div>

  {#if error}<p class="err">{error}</p>{/if}
  <div class="actions">
    <button class="save" disabled={saving} onclick={save}>{t('Save the day')}</button>
    {#if saved}<span class="ok">{t('Saved')}</span>{/if}
  </div>
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
  .dim {
    color: var(--ink-dim);
    line-height: 1.5;
    font-size: 0.9rem;
    margin: 0 0 0.9rem;
  }
  .small {
    font-size: 0.82rem;
    margin-bottom: 0.5rem;
  }
  .lab {
    display: block;
    font-size: 0.72rem;
    font-weight: 800;
    letter-spacing: 0.1em;
    text-transform: uppercase;
    color: var(--ink-dim);
    margin-bottom: 0.35rem;
  }
  .block {
    display: block;
    margin-bottom: 1rem;
  }
  textarea {
    width: 100%;
    resize: vertical;
    font: inherit;
    line-height: 1.5;
    padding: 0.5rem 0.6rem;
  }

  .sched {
    margin-bottom: 1rem;
  }
  .row {
    display: grid;
    grid-template-columns: 5.5rem 1fr 7rem auto;
    gap: 0.4rem;
    margin-bottom: 0.35rem;
    align-items: center;
  }
  .row input,
  .row select {
    padding: 0.35rem 0.45rem;
    font-size: 0.9rem;
    min-width: 0;
  }
  .rowbtns {
    display: inline-flex;
    gap: 0.2rem;
  }
  .rowbtns button {
    padding: 0.25rem 0.45rem;
    font-size: 0.85rem;
    background: var(--panel-2);
    border: 1px solid var(--line);
    color: var(--ink-dim);
  }
  .add {
    margin: 0.5rem 0 0;
    display: flex;
    flex-wrap: wrap;
    gap: 0.4rem;
  }
  .add button {
    padding: 0.35rem 0.7rem;
    font-size: 0.85rem;
    background: var(--panel-2);
    border: 1px solid var(--line);
  }

  .fields {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(9rem, 1fr));
    gap: 0.6rem;
  }
  .fields label {
    display: grid;
    gap: 0.3rem;
    font-size: 0.85rem;
    color: var(--ink-dim);
  }
  .fields input,
  .fields select {
    padding: 0.4rem 0.5rem;
    min-width: 0;
  }

  .actions {
    display: flex;
    align-items: center;
    gap: 0.8rem;
    margin-top: 1rem;
  }
  .save {
    padding: 0.55rem 1rem;
    font-weight: 700;
    background: var(--ink);
    color: #0d0f14;
    border: none;
  }
  .save:disabled {
    opacity: 0.5;
  }
  .ok {
    color: var(--ok);
    font-size: 0.88rem;
  }
  .err {
    margin: 0.5rem 0 0;
    color: var(--amber-bright);
  }

  /* The schedule rows stack rather than squeeze on a phone: an organizer does sometimes
     fix the programme from the same tablet they are carrying round the hall. */
  @media (max-width: 40rem) {
    .row {
      grid-template-columns: 5rem 1fr;
      grid-template-areas:
        'at what'
        'kind btns';
      row-gap: 0.3rem;
      padding-bottom: 0.5rem;
      border-bottom: 1px solid var(--line);
    }
    .row .at {
      grid-area: at;
    }
    .row .what {
      grid-area: what;
    }
    .row select {
      grid-area: kind;
    }
    .rowbtns {
      grid-area: btns;
      justify-content: flex-end;
    }
  }
</style>
