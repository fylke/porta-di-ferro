<script lang="ts">
  import { onMount, untrack } from 'svelte';
  import {
    api,
    type EventInfo,
    type EventSignupPreview,
    type EventSignupReady,
    type EventSignupRow,
    type Snapshot,
    type SignupInfo,
    type SignupPreview,
    type SignupReady,
    type StaffMember,
  } from '../api';
  import { t } from '../lib/i18n.svelte';
  import { apiBase } from '../lib/paths';
  import { discipline } from '../router.svelte';

  /**
   * Offline signup, the organizer's half (issue #91).
   *
   * One file goes out with the event written into it; a folder of files comes back and is
   * imported here. Nothing in between touches a network, which is the point: the project
   * exists so a club can run an event with no internet-facing server, and registration
   * was the last part that still meant typing forty names off a spreadsheet.
   *
   * Importing is two steps on screen because it is two calls on the server. Nothing is
   * written until the organizer has seen what would be, and the confirm re-checks rather
   * than trusting what this screen sends back.
   */
  /**
   * Where it runs. 'single': a discipline on its own, or an event's only one, as it always
   * was. 'event': the event's admin, where one import gives every discipline its share
   * (phase 3). 'share': a discipline of an event with several, which keeps its staff and
   * points at the event's import.
   */
  let {
    snapshot,
    info,
    onchange,
    scope = 'single',
  }: {
    snapshot?: Snapshot;
    /** The event's day, for the event's panel. */
    info?: EventInfo;
    onchange: () => void;
    scope?: 'single' | 'event' | 'share';
  } = $props();

  // The files are the event's, or this discipline's: the definition, and the app with it
  // baked in.
  // svelte-ignore state_referenced_locally
  const base = scope === 'event' ? '/api/event' : apiBase(discipline());
  // svelte-ignore state_referenced_locally
  const several = scope === 'event';

  const initial = untrack(() => (scope === 'event' ? info?.signup : snapshot?.tournament.event?.signup) ?? {});
  let definitionId = $state(initial.definitionId ?? '');
  let name = $state(initial.name ?? '');
  let venue = $state(initial.venue ?? '');
  let date = $state(initial.date ?? '');
  let tournament = $state(initial.tournament ?? '');
  let contact = $state(initial.contact ?? false);

  let ready = $state<SignupReady | EventSignupReady | null>(null);
  const shares = $derived(ready && 'disciplines' in ready ? ready.disciplines : []);
  const unclaimed = $derived(ready && 'unclaimed' in ready ? ready.unclaimed : []);
  let saving = $state(false);
  let saved = $state(false);
  let error = $state('');

  // The chosen files and what the server says about them. Kept apart: the organizer can
  // look, change their mind, and pick a different folder without anything having
  // happened.
  let files = $state<{ source: string; body: string }[]>([]);
  let preview = $state<SignupPreview | EventSignupPreview | null>(null);
  const rows: EventSignupRow[] = $derived(preview?.rows ?? []);
  // Over capacity, and imported after the draw: per discipline at the event's import.
  const capacity = $derived(
    !preview ? [] : 'disciplines' in preview ? preview.disciplines.flatMap((d) => d.capacity ?? []) : (preview.capacity ?? []),
  );
  const drawnInto = $derived(
    !preview
      ? []
      : 'disciplines' in preview
        ? preview.disciplines.filter((d) => d.poolsDrawn && d.adding > 0).map((d) => d.name)
        : preview.poolsDrawn && preview.adding > 0
          ? ['']
          : [],
  );
  let busy = $state(false);
  let imported = $state<{ competitors: number; staff: number } | null>(null);

  // Who has offered to work this discipline rather than fence in it (issue #5).
  const staff: StaffMember[] = $derived(snapshot?.tournament.staff ?? []);

  async function refreshReady() {
    if (scope === 'share') return;
    try {
      ready = scope === 'event' ? await api.eventSignupReady() : await api.signupReady();
    } catch {
      // The panel still works; it just cannot say what is missing.
    }
  }

  onMount(refreshReady);

  function touch() {
    saved = false;
  }

  async function save() {
    error = '';
    saving = true;
    try {
      if (scope === 'event') {
        await api.saveEventInfo({ ...(info ?? {}), signup: { definitionId, name, venue, date, contact } });
      } else {
        const signup: SignupInfo = { definitionId, name, venue, date, tournament, contact };
        await api.saveEvent({ ...(snapshot?.tournament.event ?? {}), signup });
      }
      saved = true;
      await refreshReady();
      onchange();
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    } finally {
      saving = false;
    }
  }

  /**
   * Reading a folder of responses.
   *
   * `webkitdirectory` is what makes "point at the folder they are all in" work, and
   * unlike the File System Access API it is not gated on a secure context — which
   * matters, because every device but the organizer's own opens this over plain HTTP on
   * a LAN address (AGENTS.md). `multiple` is the fallback for picking files by hand.
   */
  async function choose(list: FileList | null) {
    if (!list) return;
    error = '';
    preview = null;
    imported = null;
    const picked: { source: string; body: string }[] = [];
    for (const file of Array.from(list)) {
      // A folder chosen wholesale brings whatever else is in it. Only the JSON is a
      // candidate, and the rest is not worth showing the organizer as forty errors.
      if (!file.name.toLowerCase().endsWith('.json')) continue;
      picked.push({ source: file.webkitRelativePath || file.name, body: await file.text() });
    }
    files = picked;
    if (picked.length === 0) {
      error = t('No .json files in there.');
      return;
    }
    await look();
  }

  /** Which programme row a discipline takes, at the event's panel. */
  async function setRow(slug: string, row: string) {
    error = '';
    try {
      ready = await api.setSignupRows({ [slug]: row });
      if (files.length > 0) await look();
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    }
  }

  async function look() {
    busy = true;
    try {
      preview = scope === 'event' ? await api.previewEventSignups(files) : await api.previewSignups(files);
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    } finally {
      busy = false;
    }
  }

  async function confirm() {
    busy = true;
    error = '';
    try {
      const res = scope === 'event' ? await api.importEventSignups(files) : await api.importSignups(files);
      imported = { competitors: res.added, staff: res.addedStaff };
      preview = res.preview;
      if ('error' in res && res.error) error = res.error;
      onchange();
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    } finally {
      busy = false;
    }
  }

  async function removeStaff(id: string) {
    error = '';
    try {
      await api.removeStaff(id);
      onchange();
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    }
  }

  function clear() {
    files = [];
    preview = null;
    imported = null;
    error = '';
  }

  const verdictLabel: Record<string, string> = $derived({
    new: t('will be added'),
    staff: t('will be added as staff'),
    already: t('already imported'),
    repeat: t('the same file twice'),
    'other-event': t('another event'),
    'not-here': several ? t('no discipline takes it') : t('another discipline'),
    unknown: t('unknown discipline'),
    invalid: t('not usable'),
  });

  const roleLabel: Record<string, string> = $derived({
    'head-ref': t('Head referee'),
    'assistant-ref': t('Assistant referee'),
    'score-keeper': t('Score keeper'),
    physician: t('Physician'),
  });

  function rolesText(roles: string[] | undefined): string {
    return (roles ?? []).map((r) => roleLabel[r] ?? r).join(', ');
  }

  /** "2 competitors and 1 staff member", leaving out whichever is none. */
  function howMany(competitors: number, staffCount: number): string {
    const parts: string[] = [];
    if (competitors > 0 || staffCount === 0) {
      parts.push(competitors === 1 ? t('1 competitor') : t('{n} competitors', { n: competitors }));
    }
    if (staffCount > 0) {
      parts.push(staffCount === 1 ? t('1 staff member') : t('{n} staff', { n: staffCount }));
    }
    return parts.length === 2 ? t('{a} and {b}', { a: parts[0], b: parts[1] }) : parts[0];
  }

  // Counted for the summary line, so the organizer is not made to read forty rows to
  // find out that thirty-eight are fine.
  const counts = $derived.by(() => {
    const out: Record<string, number> = {};
    for (const row of rows) out[row.verdict] = (out[row.verdict] ?? 0) + 1;
    return out;
  });

  /** Where a response goes, at the event's import. */
  function into(row: EventSignupRow): string {
    return (row.into ?? [])
      .map((i) => (i.verdict === 'staff' ? t('{discipline} as staff', { discipline: i.name }) : i.name))
      .join(', ');
  }
</script>

<section>
  <h2>{t('Signup files')}</h2>
  {#if scope === 'share'}
  <p class="dim">
    {t('This event takes its signups once, for every discipline:')}
    <a href="/admin">{t('on the event’s admin page')}</a>.
  </p>
  {:else}
  <p class="dim">
    {t('Send people a file they fill in offline and send back. Nothing here needs the internet, and neither does what they open.')}
  </p>

  <div class="fields">
    <label>
      {t('Event identifier')}
      <input bind:value={definitionId} oninput={touch} placeholder="msl-open-2026" />
    </label>
    <label>
      {t('Event name')}
      <input bind:value={name} oninput={touch} placeholder={t('MSL Open')} />
    </label>
    <label>
      {t('Venue')}
      <input bind:value={venue} oninput={touch} placeholder={t('Linköping')} />
    </label>
    <label>
      {t('Date')}
      <input bind:value={date} oninput={touch} placeholder="2026-11-15" />
    </label>
  </div>
  <p class="dim small">
    {t('The identifier keeps a response from last year’s event out of this one. It goes in every file and cannot change once they are sent.')}
  </p>

  {#if scope === 'event'}
  <div class="shares">
    <h3>{t('Who takes what')}</h3>
    <p class="dim small">
      {t('Each discipline takes the responses for its row in the programme: the one named like it, unless you pick another.')}
    </p>
    <ul>
      {#each shares as d (d.discipline)}
        <li>
          <span class="strong">{d.name}</span>
          {#if d.error}
            <span class="warn-inline">{d.error}</span>
          {:else}
            <select
              value={d.chosen ? d.tournament : ''}
              onchange={(e) => void setRow(d.discipline, e.currentTarget.value)}
              aria-label={t('Programme row for {discipline}', { discipline: d.name })}
            >
              <option value="">
                {d.tournament && !d.chosen
                  ? t('{row}, by its name', { row: ready?.tournaments.find((tn) => tn.id === d.tournament)?.label ?? d.tournament })
                  : t('nothing')}
              </option>
              {#each ready?.tournaments ?? [] as tn (tn.id)}
                <option value={tn.id}>{tn.label}</option>
              {/each}
            </select>
          {/if}
        </li>
      {/each}
    </ul>
    {#each unclaimed as row (row.id)}
      <p class="warn">{t('No discipline takes {row}: whoever enters it would be imported nowhere.', { row: row.label })}</p>
    {/each}
  </div>
  <div class="fields">
    <label class="check">
      <input type="checkbox" bind:checked={contact} onchange={touch} />
      {t('Ask for a contact detail')}
    </label>
  </div>
  {:else}
  <div class="fields">
    <label>
      {t('This run is')}
      <select bind:value={tournament} onchange={touch}>
        <option value="">{t('the only discipline')}</option>
        {#each ready?.tournaments ?? [] as tn (tn.id)}
          <option value={tn.id}>{tn.label}</option>
        {/each}
      </select>
    </label>
    <label class="check">
      <input type="checkbox" bind:checked={contact} onchange={touch} />
      {t('Ask for a contact detail')}
    </label>
  </div>
  {/if}

  {#if error}<p class="err">{error}</p>{/if}
  <div class="actions">
    <button class="save" disabled={saving} onclick={save}>{t('Save')}</button>
    {#if saved}<span class="ok">{t('Saved')}</span>{/if}
  </div>

  {#if ready && (ready.missing?.length ?? 0) > 0}
    <p class="warn">
      {t('Before the files can go out, this still needs:')}
      {ready.missing.join(', ')}.
    </p>
  {:else if ready}
    <div class="send">
      <h3>{t('Send this out')}</h3>
      <p class="dim small">
        {t('One file, with the event already in it. They open it, fill it in and send back a small .json.')}
      </p>
      <p class="row">
        <a class="button" href="{base}/signup/app.html?download=1">{t('The signup app')}</a>
        <a class="quiet" href="{base}/signup/app.html" target="_blank" rel="noreferrer">{t('Preview it')}</a>
        <a class="quiet" href="{base}/signup/definition.json?download=1">{t('Just the definition')}</a>
      </p>
    </div>
  {/if}

  <div class="import">
    <h3>{t('Bring responses in')}</h3>
    <p class="row">
      <label class="button">
        {t('Choose a folder')}
        <!-- webkitdirectory is what makes picking a whole folder work, and it needs no
             secure context, unlike the File System Access API. -->
        <input
          type="file"
          multiple
          webkitdirectory
          onchange={(e) => void choose(e.currentTarget.files)}
        />
      </label>
      <label class="quiet">
        {t('or pick files')}
        <input
          type="file"
          multiple
          accept=".json,application/json"
          onchange={(e) => void choose(e.currentTarget.files)}
        />
      </label>
      {#if files.length > 0}
        <button class="quiet" onclick={clear}>{t('Clear')}</button>
      {/if}
    </p>

    {#if busy}
      <p class="dim">{t('Reading…')}</p>
    {/if}

    {#if preview}
      {#if imported !== null}
        <p class="ok big">
          {t('Imported: {what}.', { what: howMany(imported.competitors, imported.staff) })}
        </p>
      {/if}

      <p class="summary">
        {#each Object.entries(counts) as [verdict, n] (verdict)}
          <span class="count {verdict}">{n} {verdictLabel[verdict]}</span>
        {/each}
      </p>

      {#each capacity as warning (warning)}
        <p class="warn">{warning}</p>
      {/each}
      {#if drawnInto.length > 0 && !several}
        <p class="warn">
          {t('The pools are already drawn. Anyone imported now will not be in one until you draw again.')}
        </p>
      {:else if drawnInto.length > 0}
        <p class="warn">
          {t('The pools are already drawn in {disciplines}. Anyone imported there will not be in one until you draw again.', { disciplines: drawnInto.join(', ') })}
        </p>
      {/if}

      <div class="scroller">
        <table>
          <thead>
            <tr>
              <th class="l">{t('Name')}</th>
              <th class="l">{t('Club')}</th>
              <th class="l">{t('Entering')}</th>
              <th class="l">{t('File')}</th>
              <th class="l">{t('What happens')}</th>
            </tr>
          </thead>
          <tbody>
            {#each rows as row (row.source + row.submissionId)}
              <tr class={row.verdict}>
                <td class="l strong">{row.name || '—'}</td>
                <td class="l">{row.club || ''}</td>
                <td class="l">{(row.entries ?? []).join(', ')}</td>
                <td class="l file">{row.source}</td>
                <td class="l">
                  <span class="verdict {row.verdict}">{verdictLabel[row.verdict]}</span>
                  {#if several && (row.into?.length ?? 0) > 0}
                    <span class="dim problem">{t('into {where}', { where: into(row) })}</span>
                  {/if}
                  {#if row.verdict === 'staff'}
                    <span class="dim problem">{t('as {roles}', { roles: rolesText(row.roles) })}</span>
                  {/if}
                  {#if row.problem}<span class="dim problem">{row.problem}</span>{/if}
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>

      {#if imported === null}
        <div class="actions">
          <button
            class="save"
            disabled={busy || preview.adding + preview.addingStaff === 0}
            onclick={confirm}
          >
            {t('Import {what}', { what: howMany(preview.adding, preview.addingStaff) })}
          </button>
          {#if preview.adding + preview.addingStaff === 0}
            <span class="dim">{t('Nothing in there to add.')}</span>
          {/if}
        </div>
      {/if}
    {/if}
  </div>
  {/if}

  {#if staff.length > 0}
    <div class="staff-list">
      <h3>{t('Staff')}</h3>
      <p class="dim small">
        {t('Offered to work this discipline. Who stands where is still up to you.')}
      </p>
      <ul>
        {#each staff as member (member.id)}
          <li>
            <span class="who">
              <span class="strong">{member.name}</span>
              {#if member.club}<span class="dim-inline">{member.club}</span>{/if}
            </span>
            <span class="roles">{rolesText(member.roles)}</span>
            <button class="quiet" onclick={() => void removeStaff(member.id)}>{t('Remove')}</button>
          </li>
        {/each}
      </ul>
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
    margin: 0 0 0.4rem;
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
    margin: 0 0 0.9rem;
  }
  .small {
    font-size: 0.82rem;
  }
  .fields {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(10rem, 1fr));
    gap: 0.6rem;
    margin-bottom: 0.6rem;
  }
  .fields label {
    display: grid;
    gap: 0.3rem;
    font-size: 0.85rem;
    color: var(--ink-dim);
    min-width: 0;
  }
  .fields input,
  .fields select {
    padding: 0.4rem 0.5rem;
    min-width: 0;
  }
  .fields label.check {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    align-self: end;
    padding-bottom: 0.5rem;
  }
  .fields label.check input {
    width: 1.1rem;
    height: 1.1rem;
  }

  .shares ul {
    list-style: none;
    margin: 0 0 0.6rem;
    padding: 0;
    display: grid;
    gap: 0.4rem;
  }
  .shares li {
    display: flex;
    flex-wrap: wrap;
    gap: 0.3rem 0.8rem;
    align-items: center;
    font-size: 0.9rem;
  }
  .shares li .strong {
    min-width: 12rem;
  }
  .shares select {
    padding: 0.35rem 0.5rem;
    min-width: 0;
    max-width: 100%;
  }
  .warn-inline {
    color: var(--amber-bright);
    font-size: 0.85rem;
  }
  .shares,
  .send,
  .import,
  .staff-list {
    margin-top: 1.2rem;
    padding-top: 1rem;
    border-top: 1px solid var(--line);
  }
  .row {
    display: flex;
    flex-wrap: wrap;
    gap: 0.5rem;
    align-items: center;
    margin: 0 0 0.6rem;
  }
  /* The file inputs are hidden inside their labels: a bare file input cannot be styled
     and reads as an afterthought on a page of buttons. */
  .button,
  .quiet {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
    padding: 0.5rem 0.9rem;
    border-radius: var(--radius);
    font-size: 0.9rem;
    font-weight: 700;
    cursor: pointer;
    text-decoration: none;
  }
  .button {
    background: var(--ink);
    color: #0d0f14;
    border: none;
  }
  .quiet {
    background: var(--panel-2);
    color: var(--ink);
    border: 1px solid var(--line);
    font-weight: 600;
  }
  .button input,
  .quiet input {
    display: none;
  }

  .actions {
    display: flex;
    align-items: center;
    gap: 0.8rem;
    margin-top: 0.9rem;
    flex-wrap: wrap;
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
  .ok.big {
    font-size: 1rem;
    font-weight: 700;
  }
  .err,
  .warn {
    margin: 0.5rem 0 0;
    color: var(--amber-bright);
    line-height: 1.5;
    font-size: 0.9rem;
  }

  .summary {
    display: flex;
    flex-wrap: wrap;
    gap: 0.4rem 1rem;
    margin: 0.8rem 0 0.4rem;
    font-size: 0.85rem;
    color: var(--ink-dim);
  }
  .count.new,
  .count.staff {
    color: var(--ok);
    font-weight: 700;
  }

  .scroller {
    overflow-x: auto;
    margin-top: 0.6rem;
  }
  table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.85rem;
    min-width: 30rem;
  }
  th,
  td {
    padding: 0.35rem 0.4rem;
    text-align: left;
    border-bottom: 1px solid var(--line);
    vertical-align: top;
  }
  th {
    color: var(--ink-dim);
    font-weight: 500;
    font-size: 0.72rem;
  }
  .l {
    text-align: left;
  }
  .strong {
    font-weight: 700;
  }
  .file {
    color: var(--ink-dim);
    font-size: 0.78rem;
    overflow-wrap: anywhere;
  }
  /* A row that will not be imported is dimmed rather than coloured: most of them are not
     errors, they are somebody else's discipline or a file already dealt with. */
  tr.already,
  tr.repeat,
  tr.not-here {
    color: var(--ink-dim);
  }
  .verdict {
    display: block;
    font-size: 0.78rem;
  }
  .verdict.new,
  .verdict.staff {
    color: var(--ok);
    font-weight: 700;
  }
  .verdict.invalid,
  .verdict.unknown,
  .verdict.other-event {
    color: var(--amber-bright);
  }
  .staff-list ul {
    list-style: none;
    margin: 0;
    padding: 0;
  }
  .staff-list li {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.3rem 0.8rem;
    padding: 0.4rem 0;
    border-bottom: 1px solid var(--line);
    font-size: 0.9rem;
  }
  .staff-list .who {
    flex: 1 1 10rem;
    min-width: 0;
  }
  .staff-list .roles {
    color: var(--ink-dim);
    font-size: 0.82rem;
  }
  .dim-inline {
    color: var(--ink-dim);
    margin-left: 0.4rem;
  }
  .problem {
    display: block;
    font-size: 0.75rem;
    overflow-wrap: anywhere;
  }
</style>
