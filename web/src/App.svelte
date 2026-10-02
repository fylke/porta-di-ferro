<script lang="ts">
  import { onMount } from 'svelte';
  import { discipline, route, query } from './router.svelte';
  import { hall } from './lib/event.svelte';
  import Organizer from './routes/Organizer.svelte';
  import EventAdmin from './routes/EventAdmin.svelte';
  import EventLanding from './routes/EventLanding.svelte';
  import DisciplinePicker from './routes/DisciplinePicker.svelte';
  import Participant from './routes/Participant.svelte';
  import Person from './routes/Person.svelte';
  import Info from './routes/Info.svelte';
  import ScoreKeeperEntry from './routes/ScoreKeeperEntry.svelte';
  import ScoreKeeper from './routes/ScoreKeeper.svelte';
  import DisplayMat from './routes/DisplayMat.svelte';
  import DisplayMats from './routes/DisplayMats.svelte';
  import DisplayRoster from './routes/DisplayRoster.svelte';
  import PrintPools from './routes/PrintPools.svelte';
  import Audience from './routes/Audience.svelte';
  import DisplayAssigned from './routes/DisplayAssigned.svelte';
  import { t } from './lib/i18n.svelte';

  // The demo build says so on every screen (issue #88). Imported dynamically and behind
  // a constant: VITE_DEMO is substituted at build time, so a normal build evaluates this
  // to false and Rollup drops the banner and the whole wasm adapter behind it. A static
  // import would have shipped the demo's loader inside the real application.
  const demo = import.meta.env.VITE_DEMO === 'true';

  // One bundle, every surface. The route decides what renders; the Go server falls
  // through to index.html so each of these is reachable by typing it in.
  const matMatch = $derived(route('/display/mat/:n'));
  const audienceMatch = $derived(route('/display/audience/:n'));
  const scoreMatch = $derived(route('/score/:mat'));
  const personMatch = $derived(route('/who/:id'));

  // One event, many disciplines (docs/proposals/one-event-many-disciplines.md §6). A page
  // with a discipline in its address -- /d/open-sabre/score/1 -- is that discipline's, and
  // every route below is matched with the prefix taken off. A page without one is the
  // event's own while the event runs several disciplines, and the one discipline's, as it
  // always was, while it runs one. Which of the two is known only from the event, so the
  // unprefixed pages wait for a first look at it: a few milliseconds on the LAN, a cached
  // copy or at most a few seconds without it.
  const slug = $derived(discipline());
  const eventPage = $derived(!slug && hall.multi);
  // The mats are the event's (phase 2): the score keeper and the mat displays are pages of
  // the hall, never of one discipline, so their addresses never ask which discipline.
  const matPage = $derived(
    !!scoreMatch || !!matMatch || !!audienceMatch || route('/score') !== null || route('/display') !== null ||
      route('/display/mats') !== null,
  );
  onMount(() => void hall.load());
</script>

{#if !slug && !hall.ready}
  <!-- The first look at the event. -->
{:else if eventPage && route('/')}
  <EventLanding />
{:else if eventPage && route('/admin')}
  <EventAdmin />
{:else if route('/')}
  <!-- The landing page is the competitors' and the spectators' (issue #98). What used to
       be here is at /admin: the address in the hall is on a poster by the door, and
       everyone who scans it lands somewhere they cannot break anything. -->
  <Participant />
{:else if route('/admin')}
  <Organizer multi={hall.multi} />
{:else if route('/info')}
  <Info />
{:else if eventPage && personMatch?.id.startsWith('pr-')}
  <!-- A person is the event's, whichever disciplines they entered (phase 3). -->
  <Person id={personMatch.id} />
{:else if eventPage && !matPage}
  <DisciplinePicker />
{:else if personMatch}
  <Person id={personMatch.id} />
{:else if route('/score')}
  <ScoreKeeperEntry />
{:else if scoreMatch}
  <ScoreKeeper mat={Number(scoreMatch.mat)} variant={query().get('variant') ?? 'panels'} />
{:else if matMatch}
  <DisplayMat mat={Number(matMatch.n)} />
{:else if audienceMatch}
  <Audience mat={Number(audienceMatch.n)} />
{:else if route('/display')}
  <DisplayAssigned />
{:else if route('/display/mats')}
  <DisplayMats ids={query().get('ids') ?? ''} />
{:else if route('/display/roster')}
  <DisplayRoster />
{:else if route('/print/pools')}
  <PrintPools />
{:else}
  <main class="missing">
    <h1>{t('Nothing here')}</h1>
    <p>
      {t('Try the')} <a href="/">{t('landing page')}</a>{t(', a mat display such as')}
      <code>/display/mat/1</code>{t(', or the roster at')} <code>/display/roster</code>.
    </p>
  </main>
{/if}

{#if demo}
  {#await import('./demo/DemoBanner.svelte') then banner}
    <banner.default />
  {/await}
{/if}

<style>
  .missing {
    max-width: 34rem;
    margin: 4rem auto;
    padding: 0 1.5rem;
    line-height: 1.6;
  }
  code {
    background: var(--panel-2);
    padding: 0.15em 0.4em;
    border-radius: 4px;
  }
</style>
