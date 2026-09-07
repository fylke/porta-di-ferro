<script lang="ts">
  import { api, type Competitor } from '../api';

  /**
   * Registration and status. Name and club only -- picture, phone number and club crest
   * are Milestone 3, and push notifications are out entirely: they need internet, which a
   * LAN-only server does not have (design §9, issue #1).
   */
  let { competitors, poolsDrawn, onchange }: {
    competitors: Competitor[];
    poolsDrawn: boolean;
    onchange: () => void;
  } = $props();

  let name = $state('');
  let club = $state('');
  let error = $state('');

  const active = $derived(competitors.filter((c) => !c.withdrawn).length);

  async function add(event: SubmitEvent) {
    event.preventDefault();
    error = '';
    try {
      await api.addCompetitor(name, club);
      name = '';
      // The club usually repeats down a queue of people signing in together, so it stays.
      onchange();
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    }
  }

  async function setWithdrawn(c: Competitor, withdrawn: boolean) {
    error = '';
    try {
      await api.updateCompetitor(c.id, { withdrawn });
      onchange();
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    }
  }

  async function remove(c: Competitor) {
    error = '';
    try {
      await api.removeCompetitor(c.id);
      onchange();
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    }
  }
</script>

<section>
  <h2>Competitors <span class="count">{active} entered</span></h2>

  <form onsubmit={add}>
    <input bind:value={name} placeholder="Name" required aria-label="Competitor name" />
    <input bind:value={club} placeholder="Club" aria-label="Club" />
    <button type="submit">Add</button>
  </form>
  {#if error}<p class="err">{error}</p>{/if}

  <ul>
    {#each competitors as c (c.id)}
      <li class:withdrawn={c.withdrawn}>
        <span class="name">{c.name}</span>
        <span class="club">{c.club}</span>
        {#if c.withdrawn}
          <span class="tag">Withdrawn</span>
          <button class="link" onclick={() => setWithdrawn(c, false)}>Reinstate</button>
        {:else if poolsDrawn}
          <button class="link" onclick={() => setWithdrawn(c, true)}>Withdraw</button>
        {:else}
          <button class="link" onclick={() => remove(c)}>Remove</button>
        {/if}
      </li>
    {/each}
  </ul>
  {#if poolsDrawn}
    <p class="hint">
      Pools are drawn, so competitors can only be withdrawn from here. A withdrawal voids
      their results as though they never entered, and everyone else's standings recompute.
    </p>
  {/if}
</section>

<style>
  section {
    background: var(--panel);
    border-radius: var(--radius);
    padding: 1.1rem 1.25rem;
  }
  h2 {
    margin: 0 0 0.9rem;
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
  form {
    display: grid;
    grid-template-columns: 1fr 1fr auto;
    gap: 0.5rem;
    margin-bottom: 0.9rem;
  }
  form button {
    padding: 0.55rem 1.1rem;
    font-weight: 700;
    background: var(--panel-2);
    border: 1px solid var(--line);
  }
  ul {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    gap: 0.2rem;
    max-height: 22rem;
    overflow-y: auto;
  }
  li {
    display: grid;
    grid-template-columns: 1fr auto auto;
    gap: 0.6rem;
    align-items: baseline;
    padding: 0.4rem 0.5rem;
    border-radius: 6px;
  }
  li:nth-child(odd) {
    background: var(--panel-2);
  }
  li.withdrawn .name,
  li.withdrawn .club {
    text-decoration: line-through;
    color: var(--ink-dim);
  }
  .club {
    color: var(--ink-dim);
    font-size: 0.9rem;
  }
  .tag {
    font-size: 0.75rem;
    color: var(--amber-bright);
  }
  .link {
    background: none;
    border: none;
    color: var(--blue-bright);
    font-size: 0.85rem;
    padding: 0;
  }
  .hint,
  .err {
    margin: 0.9rem 0 0;
    font-size: 0.9rem;
    line-height: 1.5;
    color: var(--ink-dim);
  }
  .err {
    color: var(--amber-bright);
  }
</style>
