<script lang="ts">
  import { untrack } from 'svelte';
  import type { EventInfo } from '../api';
  import { t } from '../lib/i18n.svelte';
  import SaveButton from './SaveButton.svelte';

  /**
   * The event, the first step of planning it (#149): what it is called, where and when, and
   * the identifier that goes in every signup file -- needed before anything else, because
   * the signup and the info sheet are built from them -- and the welcome and the venue
   * wifi for the info sheet by the door and every phone in the hall.
   *
   * The programme is not here any more: it is generated from the plan (#149), so nobody
   * types "pools at 10:40" and then forgets to change it.
   *
   * Saved on a button rather than as you type. The welcome is a paragraph, and an autosave
   * that fires mid-sentence would push half-written text to every screen in the venue.
   */
  let {
    event,
    save: send,
    onchange,
  }: { event: EventInfo; save: (event: EventInfo) => Promise<unknown>; onchange: () => void } = $props();

  // Seeded once: the organizer is the only writer, and a field that rewrites itself under
  // the cursor is worse than one briefly stale.
  const initial = untrack(() => event);
  let definitionId = $state(initial.signup?.definitionId ?? '');
  let name = $state(initial.signup?.name ?? '');
  let venue = $state(initial.signup?.venue ?? '');
  let date = $state(initial.signup?.date ?? '');
  let welcome = $state(initial.welcome ?? '');
  let ssid = $state(initial.wifi?.ssid ?? '');
  let password = $state(initial.wifi?.password ?? '');
  let security = $state(initial.wifi?.security ?? 'WPA');
  // The identifier follows the name and the year until somebody types one of their own.
  let ownId = $state(!!initial.signup?.definitionId);

  let saving = $state(false);
  let error = $state('');
  let dirty = $state(false);

  function touch() {
    dirty = true;
    error = '';
    if (!ownId) definitionId = suggestId(name, date);
  }

  /** "Autumn Open" on 2026-11-15 is autumn-open-2026. */
  function suggestId(n: string, d: string): string {
    const slug = n
      .toLocaleLowerCase()
      .normalize('NFD')
      .replace(/[̀-ͯ]/g, '')
      .replace(/[^a-z0-9]+/g, '-')
      .replace(/^-|-$/g, '');
    const year = /^\d{4}/.exec(d.trim())?.[0];
    return slug && year && !slug.endsWith(year) ? `${slug}-${year}` : slug;
  }

  /** What still has to be filled in before the event can be planned further. */
  const missing = $derived(
    [
      [definitionId, t('Event identifier')],
      [name, t('Event name')],
      [venue, t('Venue')],
      [date, t('Date')],
    ]
      .filter(([v]) => !v.trim())
      .map(([, label]) => label),
  );

  async function save(): Promise<boolean> {
    error = '';
    saving = true;
    try {
      // On top of the event as it stands now, not as it was when the page opened: the save
      // replaces the whole of it, and the signup settings are edited elsewhere too.
      const next: EventInfo = {
        ...event,
        welcome,
        signup: { ...event.signup, definitionId: definitionId.trim(), name: name.trim(), venue: venue.trim(), date: date.trim() },
        wifi: { ssid, password, security: security as 'WPA' | 'WEP' | 'nopass' },
      };
      await send(next);
      dirty = false;
      onchange();
      return true;
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
      return false;
    } finally {
      saving = false;
    }
  }

  /**
   * For the wizard's Next: saves what is unsaved, and says whether the event is complete
   * enough to go on. The four that go in every signup file are required (#149).
   */
  export async function commit(): Promise<boolean> {
    if (missing.length > 0) {
      error = t('Fill in {fields} to go on.', { fields: missing.join(', ') });
      return false;
    }
    return dirty ? await save() : true;
  }
</script>

<section data-tour="plan-event">
  <h2>{t('The event')}</h2>
  <p class="dim">{t('What it is called, where and when. These four go in every signup file, so they are needed before anything else.')}</p>

  <div class="fields">
    <label>
      <span>{t('Event name')} <span class="req">*</span></span>
      <input bind:value={name} oninput={touch} placeholder={t('MSL Open')} required />
    </label>
    <label>
      <span>{t('Venue')} <span class="req">*</span></span>
      <input bind:value={venue} oninput={touch} placeholder={t('Linköping')} required />
    </label>
    <label>
      <span>{t('Date')} <span class="req">*</span></span>
      <input type="date" bind:value={date} oninput={touch} required />
    </label>
    <label>
      <span>{t('Event identifier')} <span class="req">*</span></span>
      <input bind:value={definitionId} oninput={() => ((ownId = true), touch())} placeholder="msl-open-2026" required />
    </label>
  </div>
  <p class="dim small">
    {t('The identifier keeps a response from last year’s event out of this one. It goes in every file and cannot change once they are sent.')}
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
    <SaveButton {dirty} busy={saving} onclick={() => void save()} />
    <a href="/info" target="_blank" rel="noreferrer">{t('See the info sheet')}</a>
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
    margin: 0.4rem 0 1rem;
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
  .req {
    color: var(--amber-bright);
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
  .fields {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(11rem, 1fr));
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
  .err {
    margin: 0.5rem 0 0;
    color: var(--amber-bright);
  }
</style>
