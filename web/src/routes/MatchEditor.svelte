<script lang="ts">
  import { onMount } from 'svelte';
  import { api, type MatchView } from '../api';
  import { MSL, replay, type Event, type Reason, type TimerAction } from '../lib/match';
  import { formatClock } from '../lib/clock.svelte';

  /**
   * Full history editing (design §7 item 1): the organizer corrects any entry in a match
   * log, running or finished, and the scores, standings and bracket follow.
   *
   * This is the one deliberate exception to the log being append-only. The score keeper's
   * undo appends a correction; this rewrites the log, because a log the organizer has
   * corrected should read as the match that happened, not as the match plus a trail of
   * what was thought to have happened. What makes that safe is that nothing is lost: the
   * server keeps the version being replaced as a dated backup beside the log, every time.
   *
   * The state at the bottom is the client engine replaying the edited log as it stands,
   * so the organizer sees what a change does before saving it.
   */
  let {
    match,
    names,
    onClose,
    onSaved,
  }: {
    match: MatchView;
    names: { red: string; blue: string };
    onClose: () => void;
    onSaved: () => void;
  } = $props();

  let rows = $state<Event[]>([]);
  let backups = $state<string[]>([]);
  let loading = $state(true);
  let saving = $state(false);
  let error = $state('');
  let dirty = $state(false);

  onMount(() => {
    void (async () => {
      try {
        rows = await api.events(match.id, 0);
        backups = await api.backups(match.id);
      } catch (e) {
        error = e instanceof Error ? e.message : String(e);
      } finally {
        loading = false;
      }
    })();
  });

  const preview = $derived(replay(MSL, rows));
  const timerActions: TimerAction[] = ['start', 'stop', 'resume', 'reset'];
  const reasons: Reason[] = ['time', 'point_cap', 'penalty', 'forfeit'];

  function touch() {
    dirty = true;
  }

  function remove(i: number) {
    rows = rows.filter((_, k) => k !== i);
    touch();
  }

  function move(i: number, by: -1 | 1) {
    const j = i + by;
    if (j < 0 || j >= rows.length) return;
    const next = [...rows];
    [next[i], next[j]] = [next[j], next[i]];
    rows = next;
    touch();
  }

  function addExchange() {
    const last = rows[rows.length - 1];
    const at = last ? last.elapsedMs : 0;
    const fresh: Event = {
      seq: rows.length + 1,
      type: 'exchange',
      elapsedMs: at,
      exchange: { red: { value: 0, penalty: 0 }, blue: { value: 0, penalty: 0 } },
    };
    // Before the end, if there is one; a match that has ended ignores what comes after.
    const endAt = rows.findIndex((r) => r.type === 'end');
    rows = endAt < 0 ? [...rows, fresh] : [...rows.slice(0, endAt), fresh, ...rows.slice(endAt)];
    touch();
  }

  function addEnd() {
    const last = rows[rows.length - 1];
    rows = [...rows, { seq: rows.length + 1, type: 'end', elapsedMs: last?.elapsedMs ?? 0, end: { reason: 'time' } }];
    touch();
  }

  function setSeconds(row: Event, value: string) {
    const s = Number(value);
    if (Number.isFinite(s) && s >= 0) row.elapsedMs = Math.round(s * 1000);
    touch();
  }

  async function save() {
    error = '';
    saving = true;
    try {
      // Sequence numbers follow position: the order on screen is the order of the match.
      const renumbered = rows.map((r, i) => ({ ...r, seq: i + 1 }));
      await api.replaceEvents(match.id, renumbered);
      onSaved();
      onClose();
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    } finally {
      saving = false;
    }
  }

  const hasEnd = $derived(rows.some((r) => r.type === 'end'));
</script>

<div class="scrim" role="dialog" aria-modal="true" aria-label="Edit the match log">
  <div class="card">
    <header>
      <h2>Edit the log &middot; <span class="red">{names.red}</span> v <span class="blue">{names.blue}</span></h2>
      <button class="close" onclick={onClose} aria-label="Close">&times;</button>
    </header>

    {#if loading}
      <p class="dim">Loading&hellip;</p>
    {:else}
      <table>
        <thead>
          <tr>
            <th>#</th>
            <th>Type</th>
            <th>Clock (s)</th>
            <th class="red">{names.red}</th>
            <th class="blue">{names.blue}</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          {#each rows as row, i (i)}
            <tr class={row.type}>
              <td class="mono dim">{i + 1}</td>
              <td>
                {row.type}
                {#if row.type === 'undo' && row.undo}<span class="dim">of #{row.undo.seq}</span>{/if}
              </td>
              <td>
                <input
                  type="number"
                  step="0.1"
                  min="0"
                  value={(row.elapsedMs / 1000).toFixed(1)}
                  onchange={(e) => setSeconds(row, e.currentTarget.value)}
                />
                <span class="dim mono">{formatClock(row.elapsedMs)}</span>
              </td>
              {#if row.type === 'exchange' && row.exchange}
                <td>
                  <label>pts <input type="number" min="0" max={MSL.maxValue} bind:value={row.exchange.red.value} oninput={touch} /></label>
                  <label>warn <input type="number" min="0" max="3" bind:value={row.exchange.red.penalty} oninput={touch} /></label>
                </td>
                <td>
                  <label>pts <input type="number" min="0" max={MSL.maxValue} bind:value={row.exchange.blue.value} oninput={touch} /></label>
                  <label>warn <input type="number" min="0" max="3" bind:value={row.exchange.blue.penalty} oninput={touch} /></label>
                </td>
              {:else if row.type === 'timer' && row.timer}
                <td colspan="2">
                  <select bind:value={row.timer.action} onchange={touch}>
                    {#each timerActions as a (a)}<option value={a}>{a}</option>{/each}
                  </select>
                </td>
              {:else if row.type === 'end' && row.end}
                <td colspan="2">
                  <select bind:value={row.end.reason} onchange={touch}>
                    {#each reasons as r (r)}<option value={r}>{r.replace('_', ' ')}</option>{/each}
                  </select>
                  {#if row.end.reason === 'forfeit'}
                    <select bind:value={row.end.forfeiter} onchange={touch}>
                      <option value="red">{names.red} forfeits</option>
                      <option value="blue">{names.blue} forfeits</option>
                    </select>
                  {/if}
                </td>
              {:else if row.type === 'options' && row.options}
                <td colspan="2" class="dim">{row.options.red} / {row.options.blue}{row.options.swapDisplay ? ' · display swapped' : ''}</td>
              {:else}
                <td colspan="2"></td>
              {/if}
              <td class="actions">
                <button title="Earlier" aria-label="Move up" onclick={() => move(i, -1)}>&uarr;</button>
                <button title="Later" aria-label="Move down" onclick={() => move(i, 1)}>&darr;</button>
                <button title="Delete this event" aria-label="Delete" onclick={() => remove(i)}>&times;</button>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>

      <p class="add">
        <button onclick={addExchange}>Add an exchange</button>
        {#if !hasEnd}<button onclick={addEnd}>Add the end</button>{/if}
      </p>

      <div class="preview">
        <span class="label">Replays to</span>
        <span class="mono score">{preview.red.score}&ndash;{preview.blue.score}</span>
        <span class="dim">
          {#if preview.ended}
            ended &middot; {preview.winner ? `${preview.winner === 'red' ? names.red : names.blue} wins` : 'draw'}
            {#if preview.endReason}({preview.endReason.replace('_', ' ')}){/if}
          {:else}
            not ended{#if preview.pending !== 'none'} &middot; pending {preview.pending.replace('_', ' ')}{/if}
          {/if}
          &middot; warnings {preview.red.penalty}/{preview.blue.penalty}
        </span>
      </div>

      <p class="warn">
        Saving rewrites this match's log. The version being replaced is kept as a backup beside
        it{backups.length > 0 ? ` (${backups.length} so far)` : ''}. A score keeper holding the match
        reloads it.
      </p>
      {#if error}<p class="err">{error}</p>{/if}

      <div class="buttons">
        <button class="save" disabled={saving || !dirty} onclick={save}>Save the edited log</button>
        <button onclick={onClose}>Cancel</button>
      </div>
    {/if}
  </div>
</div>

<style>
  .scrim {
    position: fixed;
    inset: 0;
    background: rgba(6, 8, 12, 0.86);
    display: grid;
    place-items: center;
    padding: 1rem;
    z-index: 50;
  }
  .card {
    background: var(--panel);
    border: 1px solid var(--line);
    border-radius: 14px;
    padding: 1.25rem 1.5rem;
    width: min(60rem, 100%);
    max-height: 100%;
    overflow: auto;
    display: grid;
    gap: 0.9rem;
  }
  header {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }
  h2 {
    margin: 0;
    font-size: 1.15rem;
  }
  .close {
    background: none;
    border: none;
    font-size: 1.6rem;
    line-height: 1;
    color: var(--ink-dim);
  }
  table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.88rem;
  }
  th,
  td {
    padding: 0.35rem 0.4rem;
    text-align: left;
    border-bottom: 1px solid var(--line);
    vertical-align: middle;
  }
  th {
    font-size: 0.7rem;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--ink-dim);
    font-weight: 600;
  }
  tr.end td {
    background: var(--panel-2);
  }
  td label {
    display: inline-flex;
    align-items: center;
    gap: 0.25rem;
    margin-right: 0.5rem;
    font-size: 0.75rem;
    color: var(--ink-dim);
  }
  input[type='number'] {
    width: 4.2rem;
    padding: 0.25rem 0.4rem;
    font-size: 0.85rem;
  }
  select {
    padding: 0.25rem 0.4rem;
    font-size: 0.85rem;
  }
  .actions {
    white-space: nowrap;
    text-align: right;
  }
  .actions button {
    padding: 0.2rem 0.5rem;
    font-size: 0.85rem;
    background: var(--panel-2);
    border: 1px solid var(--line);
  }
  .add {
    margin: 0;
    display: flex;
    gap: 0.5rem;
  }
  .add button,
  .buttons button {
    padding: 0.5rem 0.9rem;
    font-size: 0.9rem;
    background: var(--panel-2);
    border: 1px solid var(--line);
  }
  .preview {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: 0.6rem;
    padding: 0.6rem 0.8rem;
    border-radius: var(--radius);
    background: var(--panel-2);
  }
  .label {
    font-size: 0.7rem;
    letter-spacing: 0.1em;
    text-transform: uppercase;
    color: var(--ink-dim);
  }
  .score {
    font-size: 1.3rem;
    font-weight: 800;
  }
  .red {
    color: var(--red-bright);
  }
  .blue {
    color: var(--blue-bright);
  }
  .dim {
    color: var(--ink-dim);
  }
  .warn {
    margin: 0;
    font-size: 0.88rem;
    line-height: 1.5;
    color: var(--amber-bright);
  }
  .err {
    margin: 0;
    color: var(--amber-bright);
    font-weight: 700;
  }
  .buttons {
    display: flex;
    gap: 0.6rem;
    justify-content: flex-end;
  }
  .save {
    background: var(--ink) !important;
    color: #0d0f14 !important;
    border-color: var(--ink) !important;
    font-weight: 700;
  }
  .save:disabled {
    opacity: 0.4;
  }
</style>
