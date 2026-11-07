<script lang="ts">
  import type { ProgrammeRow } from '../api';
  import { t } from '../lib/i18n.svelte';
  import Programme from './Programme.svelte';

  /**
   * What a page looks like with something on it, for the guided tour (#153). A new event has
   * no disciplines, so the tour's steps about them would point at an empty panel. Shown only
   * while the tour is open and the panel really is empty, marked as an example, and made of
   * text and nothing to press: the page underneath stays the real one.
   */
  let { kind }: { kind: 'disciplines' | 'breaks' | 'each' | 'programme' } = $props();

  const longsword = $derived(t('Open longsword'));
  const rapier = $derived(t('Rapier and dagger'));

  const row = (stage: ProgrammeRow['stage'], name: string, from: string, to: string, mats: number[] = [], discipline = name): ProgrammeRow => ({
    discipline,
    name,
    stage,
    start: `2026-11-14T${from}:00+01:00`,
    end: `2026-11-14T${to}:00+01:00`,
    mats,
  });
  const programme = $derived([
    row('briefing', longsword, '08:15', '08:30'),
    row('gear-check', longsword, '08:30', '08:45'),
    row('pools', longsword, '09:00', '11:40', [1, 2]),
    row('break', 'Lunch', '12:00', '12:45', [], ''),
    row('eliminations', longsword, '12:45', '14:10', [1]),
    row('final', longsword, '14:20', '14:30', [1]),
    row('briefing', rapier, '14:00', '14:15'),
    row('gear-check', rapier, '14:15', '14:30'),
    row('pools', rapier, '14:45', '16:20', [2]),
    row('eliminations', rapier, '16:30', '17:10', [2]),
    row('final', rapier, '17:20', '17:30', [2]),
  ]);
</script>

<div class="example" inert>
  <span class="tag">{t('Example')}</span>
  {#if kind === 'disciplines'}
    <ul>
      <li><strong>{longsword}</strong> <span class="ready">{t('Set up')} ✓</span></li>
      <li><strong>{rapier}</strong> <span class="dim">{t('Not set up yet')}</span></li>
    </ul>
  {:else if kind === 'breaks'}
    <p>{t('Lunch')} &middot; 12:00&ndash;12:45</p>
  {:else if kind === 'each'}
    <table>
      <thead>
        <tr>
          <th>{t('Discipline')}</th>
          <th>{t('Expected')}</th>
          <th>{t('Mats')}</th>
          <th>{t('Gear check')}</th>
          <th>{t('Staff briefing')}</th>
        </tr>
      </thead>
      <tbody>
        <tr><td>{longsword}</td><td>24</td><td>2</td><td>15</td><td>15</td></tr>
        <tr><td>{rapier}</td><td>12</td><td>1</td><td>15</td><td>15</td></tr>
      </tbody>
    </table>
  {:else}
    <p class="end">{t('Estimated to end at {time}', { time: '17:30' })}</p>
    <Programme rows={programme} />
  {/if}
</div>

<style>
  .example {
    position: relative;
    margin: 0.4rem 0 0.7rem;
    padding: 0.7rem 0.8rem;
    border: 1px dashed var(--line);
    border-radius: var(--radius);
    font-size: 0.9rem;
  }
  .tag {
    display: inline-block;
    margin-bottom: 0.4rem;
    font-size: 0.68rem;
    font-weight: 800;
    letter-spacing: 0.1em;
    text-transform: uppercase;
    color: var(--amber-bright);
  }
  ul {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    gap: 0.35rem;
  }
  li {
    display: flex;
    flex-wrap: wrap;
    gap: 0.6rem;
    align-items: baseline;
  }
  p {
    margin: 0;
  }
  .end {
    font-weight: 700;
  }
  .ready {
    color: var(--ok);
  }
  .dim {
    color: var(--ink-dim);
  }
  table {
    border-collapse: collapse;
  }
  th,
  td {
    padding: 0.2rem 1rem 0.2rem 0;
    text-align: left;
  }
  th {
    color: var(--ink-dim);
    font-weight: 500;
    font-size: 0.72rem;
  }
</style>
