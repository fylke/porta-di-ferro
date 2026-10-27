<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '../api';
  import { hall } from '../lib/event.svelte';
  import { navigate, query } from '../router.svelte';
  import { t } from '../lib/i18n.svelte';
  import EventEditor from './EventEditor.svelte';
  import Disciplines from './Disciplines.svelte';
  import Planning from './Planning.svelte';
  import LangToggle from './LangToggle.svelte';

  /**
   * Plan the event (#142): what is set once, before the day, in the order it is set -- the
   * event itself, the disciplines, and how long it all takes on how many mats. The first
   * step's name, venue, date and identifier are needed to go on (#149): the signup and the
   * info sheet are built from them. The programme is not typed anywhere; it is generated
   * from the plan, and shown at the end of it.
   * It used to be half the admin page, between the mats and the screens an organizer
   * watches all day; it is here now, a step at a time, and the admin page links to it.
   *
   * The steps are addresses (/plan?step=2), so Back works and a step can be opened
   * straight from wherever it is wanted -- the landing page's Edit opens the first.
   */
  const steps = [
    { title: () => t('The event'), hint: () => t('What it is called, where and when, and what the info sheet says to everyone in the hall.') },
    { title: () => t('Disciplines'), hint: () => t('What is fenced. Set up each one: the mats it uses and the size of its pools.') },
    { title: () => t('Planning the day'), hint: () => t('How long things take, the mats, the breaks, and in what order the day runs. The programme is written from this.') },
  ];

  onMount(() => {
    void hall.refresh();
    hall.follow();
    return () => hall.unfollow();
  });

  const view = $derived(hall.view);
  // The event's four that go in every signup file: no planning past the first step without them.
  const named = $derived.by(() => {
    const s = view?.info.signup;
    return !!(s?.definitionId?.trim() && s?.name?.trim() && s?.venue?.trim() && s?.date?.trim());
  });
  const asked = $derived(Math.min(steps.length, Math.max(1, Number(query().get('step')) || 1)));
  const step = $derived(view && !named ? 1 : asked);
  let editor = $state<{ commit: () => Promise<boolean> } | undefined>();

  async function go(n: number) {
    if (step === 1 && n > 1 && editor && !(await editor.commit())) return;
    navigate(n > steps.length ? '/admin' : `/plan?step=${n}`);
    window.scrollTo(0, 0);
  }
</script>

<main class="admin">
  <header>
    <h1>
      {t('Plan the event')}
      {#if view?.name}<span class="event">{view.name}</span>{/if}
    </h1>
    <nav>
      <a href="/admin">{t('Back to the admin page')}</a>
      <LangToggle />
    </nav>
  </header>

  <ol class="steps">
    {#each steps as s, i (i)}
      <li class:current={step === i + 1}>
        {#if i === 0 || named}
          <a href="/plan?step={i + 1}" aria-current={step === i + 1 ? 'step' : undefined}>
            <span class="n">{i + 1}</span>{s.title()}
          </a>
        {:else}
          <span class="locked" title={t('Fill in the event first')}><span class="n">{i + 1}</span>{s.title()}</span>
        {/if}
      </li>
    {/each}
  </ol>
  <p class="hint">{steps[step - 1].hint()}</p>

  {#if !view}
    <p class="hint">{hall.error || t('Loading…')}</p>
  {:else if step === 1}
    <!-- Keyed on the event, so a new event after a delete is a new form; never on the
         view itself, which changes with every exchange and would lose what is typed. -->
    {#key view.id}
      <EventEditor bind:this={editor} event={view.info} save={api.saveEventInfo} onchange={() => void hall.refresh()} />
    {/key}
  {:else if step === 2}
    <div data-tour="plan-disciplines"><Disciplines setup /></div>
  {:else}
    <Planning />
  {/if}

  <div class="pager">
    {#if step > 1}
      <button class="secondary" onclick={() => go(step - 1)}>{t('Back')}</button>
    {/if}
    <button onclick={() => go(step + 1)}>
      {step < steps.length ? t('Next: {step}', { step: steps[step].title() }) : t('Done')}
    </button>
  </div>
</main>

<style>
  main {
    max-width: 68rem;
    margin: 0 auto;
    padding: 1.25rem 1.25rem 4rem;
  }
  header {
    display: flex;
    flex-wrap: wrap;
    gap: 1rem;
    justify-content: space-between;
    align-items: baseline;
    margin-bottom: 1rem;
  }
  h1 {
    margin: 0;
    font-size: 1.5rem;
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: 0.6rem;
  }
  .event {
    font-size: 1.1rem;
    font-weight: 600;
    color: var(--amber-bright);
  }
  nav {
    display: flex;
    flex-wrap: wrap;
    gap: 0.5rem 1rem;
    font-size: 0.9rem;
  }
  .steps {
    list-style: none;
    margin: 0 0 0.6rem;
    padding: 0;
    display: flex;
    flex-wrap: wrap;
    gap: 0.4rem;
  }
  .steps a,
  .steps .locked {
    display: inline-flex;
    align-items: center;
    gap: 0.45rem;
    padding: 0.35rem 0.8rem 0.35rem 0.4rem;
    border-radius: 999px;
    border: 1px solid var(--line);
    color: var(--ink-dim);
    text-decoration: none;
    font-size: 0.9rem;
  }
  .steps .locked {
    opacity: 0.5;
  }
  .steps .current a {
    border-color: var(--amber-bright);
    color: var(--ink);
    font-weight: 700;
  }
  .n {
    display: inline-grid;
    place-items: center;
    width: 1.4rem;
    height: 1.4rem;
    border-radius: 50%;
    background: var(--panel-2);
    font-size: 0.8rem;
    font-weight: 800;
  }
  .current .n {
    background: var(--amber-bright);
    color: #0d0f14;
  }
  .hint {
    margin: 0 0 1rem;
    color: var(--ink-dim);
    line-height: 1.5;
  }
  .pager {
    display: flex;
    justify-content: flex-end;
    gap: 0.6rem;
  }
</style>
