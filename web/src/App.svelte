<script lang="ts">
  import { route, query } from './router.svelte';
  import Organizer from './routes/Organizer.svelte';
  import ScoreKeeperEntry from './routes/ScoreKeeperEntry.svelte';
  import ScoreKeeper from './routes/ScoreKeeper.svelte';
  import DisplayMat from './routes/DisplayMat.svelte';
  import DisplayMats from './routes/DisplayMats.svelte';
  import DisplayRoster from './routes/DisplayRoster.svelte';
  import PrintPools from './routes/PrintPools.svelte';
  import Audience from './routes/Audience.svelte';
  import DisplayAssigned from './routes/DisplayAssigned.svelte';
  import { t } from './lib/i18n.svelte';

  // One bundle, every surface. The route decides what renders; the Go server falls
  // through to index.html so each of these is reachable by typing it in.
  const matMatch = $derived(route('/display/mat/:n'));
  const audienceMatch = $derived(route('/display/audience/:n'));
  const scoreMatch = $derived(route('/score/:mat'));
</script>

{#if route('/')}
  <Organizer />
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
      {t('Try the')} <a href="/">{t('organizer view')}</a>{t(', a mat display such as')}
      <code>/display/mat/1</code>{t(', or the roster at')} <code>/display/roster</code>.
    </p>
  </main>
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
