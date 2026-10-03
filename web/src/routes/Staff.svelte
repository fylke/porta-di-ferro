<script lang="ts">
  import { onMount } from 'svelte';
  import { api, ApiError, type StaffingView, type StaffItem, type StaffMember, type StaffSuggestion } from '../api';
  import { hall } from '../lib/event.svelte';
  import { clockOf } from '../lib/clock-of-day';
  import { itemLabel, roleLabel, ROLES, MAT_ROLES } from '../lib/items';
  import { t } from '../lib/i18n.svelte';

  /**
   * The event's staff (phase 5, #5): who has offered to work, in which roles and
   * disciplines; how many of each a mat needs; and who works each item, as the plan runs it.
   *
   * Staff arrive from the signup files or are added here by hand. A suggestion fills every
   * slot -- never with somebody fencing at the time or on another mat, and keeping people
   * in one role on one mat for as long as it can -- and changes nothing until applied. A slot
   * chosen by hand is pinned, and kept by the next suggestion; if it breaks a rule the panel
   * says so rather than undoing it.
   */
  let view = $state<StaffingView | null>(null);
  let suggestion = $state<StaffSuggestion | null>(null);
  let note = $state('');
  let error = $state('');
  let busy = $state(false);

  async function load() {
    try {
      view = await api.staff();
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    }
  }

  // The plan moves as matches finish; the times and the checks move with it.
  const entered = $derived(
    (hall.view?.disciplines ?? []).map((d) => `${d.slug}:${d.competitors}:${d.matchesDone}`).join('|'),
  );
  onMount(() => {
    void hall.refresh();
  });
  $effect(() => {
    void entered;
    void load();
  });

  const disciplines = $derived((hall.view?.disciplines ?? []).filter((d) => !d.error));
  const several = $derived(disciplines.length > 1);
  const byId = $derived(new Map((view?.members ?? []).map((m) => [m.id, m])));

  async function run(f: () => Promise<StaffingView | void>) {
    error = '';
    busy = true;
    try {
      const next = await f();
      if (next) view = next;
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    } finally {
      busy = false;
    }
  }

  // --- adding by hand ---------------------------------------------------------------------

  let name = $state('');
  let club = $state('');
  let roles = $state<string[]>(['head-ref', 'assistant-ref', 'score-keeper']);
  let works = $state<string[]>([]);
  let want = $state('');

  const fold = (s: string) =>
    s
      .toLocaleLowerCase()
      .normalize('NFD')
      .replace(/[\u0300-\u036f]/g, '')
      .trim();

  /** Somebody entered as a competitor with the name being typed: the same person, if the organizer says so. */
  const suggestions = $derived.by(() => {
    const q = fold(name);
    if (q.length < 2 || want) return [];
    const staffed = new Set((view?.members ?? []).map((m) => m.person));
    const out = new Map<string, { person: string; name: string; club?: string; in: string[] }>();
    for (const d of hall.view?.disciplines ?? []) {
      for (const e of d.entrants) {
        if (!e.person || staffed.has(e.person) || !fold(e.name).startsWith(q)) continue;
        const row = out.get(e.person);
        if (row) row.in.push(d.name);
        else out.set(e.person, { person: e.person, name: e.name, club: e.club, in: [d.name] });
      }
    }
    return [...out.values()].slice(0, 4);
  });

  function toggle(list: string[], x: string): string[] {
    return list.includes(x) ? list.filter((y) => y !== x) : [...list, x];
  }

  function add(e: SubmitEvent) {
    e.preventDefault();
    void run(async () => {
      const next = await api.addStaff({ name, club, roles, disciplines: works, ...(want ? { want } : {}) });
      name = '';
      want = '';
      return next;
    });
  }

  function update(m: StaffMember, patch: { roles?: string[]; disciplines?: string[] }) {
    void run(() => api.updateStaff(m.id, patch));
  }

  // --- the assignments ---------------------------------------------------------------------

  const crew = $derived(view?.crew ?? { 'head-ref': 1, 'assistant-ref': 2, 'score-keeper': 1 });
  /** Every slot a mat has: "head-ref" 1, "assistant-ref" 1 and 2, ... */
  const slots = $derived(MAT_ROLES.flatMap((role) => Array.from({ length: crew[role] ?? 0 }, (_, i) => ({ role, slot: i + 1 }))));

  function holder(item: string, role: string, slot: number) {
    return view?.assignments.find((a) => a.item === item && a.role === role && a.slot === slot);
  }
  function short(item: string, role: string, slot: number): boolean {
    return !!view?.short.some((s) => s.item === item && s.role === role && s.slot === slot);
  }
  function warned(item: string, role: string, staff: string | undefined): boolean {
    return !!staff && !!view?.warnings.some((w) => w.item === item && w.role === role && w.staff === staff);
  }
  /** Who could take a slot: offered the role, works the discipline. The panel says the rest. */
  function eligible(it: StaffItem, role: string): StaffMember[] {
    return (view?.members ?? []).filter(
      (m) => m.roles.includes(role) && (!m.disciplines?.length || m.disciplines.includes(it.discipline)),
    );
  }
  function assign(it: StaffItem, role: string, slot: number, staff: string) {
    void run(() => api.assign({ item: it.id, role, slot, staff }));
  }
  function setCrew(role: string, n: number) {
    void run(() => api.setCrew({ [role]: Math.max(0, Math.min(6, Math.round(n || 0))) }));
  }

  const lanes = $derived.by(() => {
    const out = new Map<string, number>();
    for (const it of view?.items ?? []) if (it.kind === 'eliminations') out.set(it.discipline, (out.get(it.discipline) ?? 0) + 1);
    return out;
  });
  function itemName(id: string): string {
    const it = view?.items.find((i) => i.id === id);
    if (!it) return id;
    const what = itemLabel(it.kind, it.number, lanes.get(it.discipline) ?? 1);
    return several ? `${it.disciplineName} ${what.toLocaleLowerCase()}` : what;
  }

  function warningText(w: StaffingView['warnings'][number]): string {
    const who = byId.get(w.staff)?.name ?? w.staff;
    const role = roleLabel(w.role).toLocaleLowerCase();
    switch (w.kind) {
      case 'fencing':
        return t('{name} is {role} on {item} while fencing in {other}.', { name: who, role, item: itemName(w.item), other: itemName(w.other ?? '') });
      case 'double':
        return t('{name} is {role} on {item} while working {other}.', { name: who, role, item: itemName(w.item), other: itemName(w.other ?? '') });
      case 'role':
        return t('{name} is {role} on {item}, which they did not offer to do.', { name: who, role, item: itemName(w.item) });
      case 'discipline':
        return t('{name} is {role} on {item}, a discipline they did not offer to work.', { name: who, role, item: itemName(w.item) });
    }
  }

  async function suggest() {
    error = '';
    busy = true;
    try {
      suggestion = await api.suggestStaff();
      note = '';
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    } finally {
      busy = false;
    }
  }
  async function apply() {
    if (!suggestion) return;
    error = '';
    busy = true;
    try {
      view = await api.applyStaff(suggestion.signature);
      suggestion = null;
    } catch (e) {
      if (e instanceof ApiError && e.status === 409) {
        suggestion = (e.body as { suggestion?: StaffSuggestion }).suggestion ?? null;
        note = t('The staff or the plan changed since; this is a new suggestion.');
      } else {
        error = e instanceof Error ? e.message : String(e);
      }
    } finally {
      busy = false;
    }
  }

  const shortCount = $derived(view?.short.length ?? 0);
  const physicians = $derived((view?.physicians ?? []).map((id) => byId.get(id)).filter((m): m is StaffMember => !!m));
</script>

<section>
  <h2>
    {t('Staff')}
    {#if view}<span class="count">{t('{n} on the staff', { n: view.members.length })}</span>{/if}
  </h2>
  <p class="dim">
    {t('Who works the mats. Staff come in with the signup files or are added here. A suggestion fills every slot, never with somebody fencing at the time or on another mat, and keeps people in one role on one mat for as long as it can.')}
  </p>
  {#if error}<p class="err" role="alert">{error}</p>{/if}

  <form class="add" onsubmit={add}>
    <label>
      {t('Name')}
      <input bind:value={name} required oninput={() => (want = '')} />
    </label>
    <label>
      {t('Club')}
      <input bind:value={club} />
    </label>
    <fieldset>
      <legend>{t('Roles')}</legend>
      {#each ROLES as r (r)}
        <label class="check"><input type="checkbox" checked={roles.includes(r)} onchange={() => (roles = toggle(roles, r))} /> {roleLabel(r)}</label>
      {/each}
    </fieldset>
    {#if several}
      <fieldset>
        <legend>{t('Works')}</legend>
        {#each disciplines as d (d.slug)}
          <label class="check"><input type="checkbox" checked={works.includes(d.slug)} onchange={() => (works = toggle(works, d.slug))} /> {d.name}</label>
        {/each}
        <span class="hint">{t('None ticked: any discipline.')}</span>
      </fieldset>
    {/if}
    <button class="save" type="submit" disabled={busy}>{t('Add to the staff')}</button>
  </form>
  {#if suggestions.length > 0}
    <div class="suggest-people">
      <p class="hint">{t('Already entered as a competitor. Is this them?')}</p>
      {#each suggestions as s (s.person)}
        <button type="button" class="pick" onclick={() => ((want = s.person), (name = s.name), (club = s.club ?? club))}>
          <span class="strong">{s.name}</span> <span class="hint">{s.club ?? ''} &middot; {s.in.join(', ')}</span>
        </button>
      {/each}
    </div>
  {/if}
  {#if want}<p class="hint">{t('Added as the same person as the competitor, so they are never put on a mat while fencing.')}</p>{/if}

  {#if view && (view.unsure ?? []).length > 0}
    <!-- Two records of one human defeat "never while fencing" (#133). -->
    <p class="warn" role="status">
      {t('{names} may be the same person as somebody else in the event. Merge them or keep them apart under People before suggesting, or a suggestion could put them on a mat while they fence.', {
        names: view.members
          .filter((m) => view?.unsure?.includes(m.id))
          .map((m) => m.name)
          .join(', '),
      })}
    </p>
  {/if}

  {#if view && view.members.length > 0}
    <div class="scroller">
      <table class="members">
        <thead>
          <tr>
            <th>{t('Name')}</th>
            {#each ROLES as r (r)}<th class="c">{roleLabel(r)}</th>{/each}
            {#if several}<th>{t('Works')}</th>{/if}
            <th></th>
          </tr>
        </thead>
        <tbody>
          {#each view.members as m (m.id)}
            <tr>
              <td>
                {#if m.person}<a href="/who/{m.person}" target="_blank" rel="noreferrer">{m.name}</a>{:else}{m.name}{/if}
                {#if m.club}<span class="hint">{m.club}</span>{/if}
                {#if view.unsure?.includes(m.id)}<span class="unsure" title={t('Maybe two people: see People')}>{t('two records?')}</span>{/if}
              </td>
              {#each ROLES as r (r)}
                <td class="c">
                  <input
                    type="checkbox"
                    aria-label={t('{name} as {role}', { name: m.name, role: roleLabel(r) })}
                    checked={m.roles.includes(r)}
                    disabled={busy}
                    onchange={() => update(m, { roles: toggle(m.roles, r) })}
                  />
                </td>
              {/each}
              {#if several}
                <td class="works">
                  {#each disciplines as d (d.slug)}
                    <label class="check small">
                      <input
                        type="checkbox"
                        checked={!m.disciplines?.length || m.disciplines.includes(d.slug)}
                        disabled={busy}
                        onchange={() => {
                          const now = m.disciplines?.length ? m.disciplines : disciplines.map((x) => x.slug);
                          update(m, { disciplines: toggle(now, d.slug) });
                        }}
                      />
                      {d.name}
                    </label>
                  {/each}
                </td>
              {/if}
              <td><button class="quiet" disabled={busy} onclick={() => run(() => api.removeMember(m.id))}>{t('Remove')}</button></td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}

  {#if physicians.length > 0}
    <p class="oncall">{t('On call as physician: {names}', { names: physicians.map((m) => m.name).join(', ') })}</p>
  {/if}

  <h3>{t('A mat needs')}</h3>
  <div class="crew">
    {#each MAT_ROLES as r (r)}
      <label>
        {roleLabel(r)}
        <input type="number" min="0" max="6" value={crew[r] ?? 0} disabled={busy} onchange={(e) => setCrew(r, Number(e.currentTarget.value))} />
      </label>
    {/each}
  </div>

  {#if view && view.items.length > 0}
    <h3>{t('Who works what')}</h3>
    <div class="head">
      <p class:warn={shortCount > 0} class:ok={shortCount === 0}>
        {shortCount === 0 ? t('Every slot is filled.') : shortCount === 1 ? t('1 slot has nobody.') : t('{n} slots have nobody.', { n: shortCount })}
      </p>
      <button class="save" disabled={busy || view.members.length === 0} onclick={suggest}>{t('Suggest who works where')}</button>
    </div>
    {#each view.warnings as w, i (i)}
      <p class="warn">{warningText(w)}</p>
    {/each}

    {#if suggestion}
      <div class="suggestion" role="region" aria-label={t('Suggested staffing')}>
        {#if note}<p class="warn">{note}</p>{/if}
        <p>
          {suggestion.changes === 0
            ? t('Nothing to change.')
            : suggestion.changes === 1
              ? t('This fills or changes 1 slot.')
              : t('This fills or changes {n} slots.', { n: suggestion.changes })}
          {#if suggestion.short.length > 0}
            {t('{n} would still have nobody: there are not enough staff free at the time.', { n: suggestion.short.length })}
          {/if}
        </p>
        <div class="actions">
          {#if suggestion.changes > 0}<button class="save" disabled={busy} onclick={apply}>{t('Apply')}</button>{/if}
          <button disabled={busy} onclick={() => (suggestion = null)}>{t('Dismiss')}</button>
        </div>
        <p class="hint">{t('Anyone chosen by hand stays where they are, and so does the crew of a mat already fencing.')}</p>
      </div>
    {/if}

    <div class="scroller">
      <table class="grid">
        <thead>
          <tr>
            <th>{t('When')}</th>
            <th>{t('Mat')}</th>
            <th>{t('What')}</th>
            {#each slots as s (`${s.role}${s.slot}`)}
              <th>{roleLabel(s.role)}{(crew[s.role] ?? 0) > 1 ? ` ${s.slot}` : ''}</th>
            {/each}
          </tr>
        </thead>
        <tbody>
          {#each view.items as it (it.id)}
            <tr class:started={it.started} class:projected={it.projected}>
              <td class="mono">{clockOf(it.start)}–{clockOf(it.end)}</td>
              <td class="mono">{it.mat}</td>
              <td>{itemName(it.id)}</td>
              {#each slots as s (`${s.role}${s.slot}`)}
                {@const a = holder(it.id, s.role, s.slot)}
                <td class:short={short(it.id, s.role, s.slot)} class:warned={warned(it.id, s.role, a?.staff)}>
                  <select
                    value={a?.staff ?? ''}
                    disabled={busy}
                    aria-label={t('{role} on {item}', { role: roleLabel(s.role), item: itemName(it.id) })}
                    onchange={(e) => assign(it, s.role, s.slot, e.currentTarget.value)}
                  >
                    <option value="">{t('nobody')}</option>
                    {#each eligible(it, s.role) as m (m.id)}<option value={m.id}>{m.name}</option>{/each}
                    {#if a && !eligible(it, s.role).some((m) => m.id === a.staff)}
                      <option value={a.staff}>{byId.get(a.staff)?.name ?? a.staff}</option>
                    {/if}
                  </select>
                  {#if a?.pinned}<span class="pin" title={t('Chosen by hand')}>&bull;</span>{/if}
                </td>
              {/each}
            </tr>
          {/each}
        </tbody>
      </table>
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
    display: flex;
    justify-content: space-between;
    align-items: baseline;
  }
  .count {
    font-size: 0.85rem;
    font-weight: 400;
    color: var(--ink-dim);
  }
  h3 {
    margin: 1.1rem 0 0.4rem;
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
    max-width: 50rem;
  }
  .hint {
    color: var(--ink-dim);
    font-size: 0.8rem;
    margin-left: 0.3rem;
  }
  .err,
  .warn {
    color: var(--amber-bright);
    font-size: 0.88rem;
    margin: 0.25rem 0;
  }
  .ok {
    color: var(--ok);
    font-size: 0.88rem;
    margin: 0;
  }
  .add {
    display: flex;
    flex-wrap: wrap;
    align-items: end;
    gap: 0.6rem 1rem;
  }
  .add label {
    display: grid;
    gap: 0.25rem;
    font-size: 0.85rem;
    color: var(--ink-dim);
  }
  .add input:not([type='checkbox']) {
    padding: 0.4rem 0.5rem;
    min-width: 0;
    width: 11rem;
    max-width: 100%;
  }
  fieldset {
    border: 1px solid var(--line);
    border-radius: 6px;
    padding: 0.3rem 0.6rem 0.4rem;
    display: flex;
    flex-wrap: wrap;
    gap: 0.2rem 0.7rem;
    font-size: 0.85rem;
  }
  legend {
    font-size: 0.75rem;
    color: var(--ink-dim);
  }
  label.check {
    display: inline-flex;
    align-items: center;
    gap: 0.3rem;
    color: var(--ink);
  }
  label.check.small {
    font-size: 0.8rem;
    margin-right: 0.5rem;
  }
  .save {
    padding: 0.5rem 1rem;
    font-weight: 700;
    background: var(--ink);
    color: #0d0f14;
    border: none;
  }
  .quiet {
    padding: 0.25rem 0.6rem;
    font-size: 0.8rem;
    background: var(--panel-2);
    border: 1px solid var(--line);
  }
  .unsure {
    margin-left: 0.4rem;
    font-size: 0.75rem;
    color: var(--amber-bright);
  }
  .suggest-people {
    margin: 0.5rem 0;
    display: flex;
    flex-wrap: wrap;
    gap: 0.4rem;
    align-items: center;
  }
  .pick {
    padding: 0.3rem 0.6rem;
    background: var(--panel-2);
    border: 1px solid var(--line);
    border-radius: 6px;
  }
  .strong {
    font-weight: 700;
  }
  .scroller {
    overflow-x: auto;
    margin-top: 0.6rem;
  }
  table {
    border-collapse: collapse;
    font-size: 0.85rem;
    width: 100%;
  }
  th,
  td {
    padding: 0.3rem 0.4rem;
    text-align: left;
    border-bottom: 1px solid var(--line);
    vertical-align: middle;
    white-space: nowrap;
  }
  th {
    color: var(--ink-dim);
    font-weight: 500;
    font-size: 0.72rem;
  }
  .c {
    text-align: center;
  }
  td.works {
    white-space: normal;
  }
  .oncall {
    margin: 0.6rem 0 0;
    font-size: 0.9rem;
  }
  .crew {
    display: flex;
    flex-wrap: wrap;
    gap: 0.6rem 1.2rem;
  }
  .crew label {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
    font-size: 0.88rem;
  }
  .crew input {
    width: 4rem;
    padding: 0.3rem 0.4rem;
  }
  .head {
    display: flex;
    flex-wrap: wrap;
    justify-content: space-between;
    align-items: center;
    gap: 0.5rem 1rem;
  }
  .suggestion {
    margin: 0.6rem 0;
    padding: 0.7rem 0.8rem;
    border: 1px dashed var(--amber-bright);
    border-radius: var(--radius);
    font-size: 0.9rem;
  }
  .suggestion p {
    margin: 0 0 0.4rem;
  }
  .actions {
    display: flex;
    gap: 0.5rem;
    margin-bottom: 0.3rem;
  }
  .grid select {
    padding: 0.2rem 0.3rem;
    font-size: 0.82rem;
    max-width: 10rem;
  }
  .grid td.short select {
    border-color: var(--amber-bright);
  }
  .grid td.warned select {
    border-color: var(--amber-bright);
    color: var(--amber-bright);
  }
  .grid tr.started td {
    opacity: 0.7;
  }
  .grid tr.projected td:nth-child(3) {
    font-style: italic;
  }
  .pin {
    margin-left: 0.2rem;
    color: var(--ink-dim);
  }
</style>
