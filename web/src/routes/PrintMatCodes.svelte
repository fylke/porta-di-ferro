<script lang="ts">
  import { onMount } from 'svelte';
  import { MatsLive } from '../lib/mats.svelte';
  import { LanAddress } from '../lib/lan.svelte';
  import { hallURL, qrSrc } from '../lib/qr';
  import { t } from '../lib/i18n.svelte';
  import LangToggle from './LangToggle.svelte';

  /**
   * A code to tape to each mat (#138): scanned, it opens the score keeper on that mat
   * straight away, with no "which mat?" to answer. The mats are the event's, so the sheet
   * is too, and it names each mat as the organizer named it.
   *
   * Printed once the hall's network is the one the tablets will join: the codes carry its
   * address.
   */
  const live = new MatsLive();
  const lan = new LanAddress();
  onMount(() => {
    void live.start();
    lan.start();
    return () => {
      lan.stop();
      live.stop();
    };
  });

  const base = $derived(hallURL(lan.clientURL));
  const mats = $derived(live.view?.mats ?? []);
</script>

<div class="sheets">
  <p class="noprint">
    <button onclick={() => window.print()} disabled={!base || mats.length === 0}>{t('Print')}</button>
    {#if base}
      {t('One code per mat, for the score keepers. Print them on the network the tablets will join: the codes carry its address.')}
    {:else}
      {t('This PC is not on a network another device could reach, so there is no address to put in the codes. Join it to the venue wifi and reload this page.')}
    {/if}
    <LangToggle compact />
  </p>

  {#if base}
    <div class="cards">
      {#each mats as m (m.mat)}
        {@const url = `${base}/score/${m.mat}`}
        <article>
          <p class="role">{t('Score keeper')}</p>
          <h1>{t('Mat {n}', { n: m.mat })}</h1>
          {#if m.name}<p class="name">{m.name}</p>{/if}
          <img src={qrSrc(url)} alt={t('QR code for {url}', { url })} />
          <p class="url">{url}</p>
          <p class="how">{t('Scan with the tablet’s camera to score this mat.')}</p>
        </article>
      {/each}
    </div>
  {/if}
</div>

<style>
  .sheets {
    background: #fff;
    color: #111;
    min-height: 100dvh;
    padding: 1rem;
  }
  .noprint {
    display: flex;
    flex-wrap: wrap;
    gap: 1rem;
    align-items: center;
    font-family: var(--font);
    color: #444;
  }
  .noprint button {
    padding: 0.5rem 1.2rem;
    font-weight: 700;
    background: #111;
    border: 1px solid #111;
    color: #fff;
  }
  .noprint button:disabled {
    opacity: 0.4;
  }
  .cards {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(min(100%, 20rem), 1fr));
    gap: 1.5rem;
    margin-top: 1.5rem;
  }
  /* Two to a page: big enough to scan from a step away, small enough to tape to a table. */
  article {
    border: 2px solid #111;
    border-radius: 12px;
    padding: 1.2rem;
    text-align: center;
    break-inside: avoid;
    page-break-inside: avoid;
  }
  .role {
    margin: 0;
    text-transform: uppercase;
    letter-spacing: 0.1em;
    font-size: 0.85rem;
    color: #444;
  }
  h1 {
    margin: 0.2rem 0 0;
    font-size: 2.6rem;
  }
  .name {
    margin: 0.1rem 0 0;
    font-size: 1.3rem;
    font-weight: 700;
  }
  img {
    display: block;
    width: min(100%, 15rem);
    height: auto;
    margin: 1rem auto 0.6rem;
  }
  .url {
    margin: 0;
    font-family: var(--mono, monospace);
    font-size: 0.85rem;
    overflow-wrap: anywhere;
  }
  .how {
    margin: 0.4rem 0 0;
    font-size: 0.85rem;
    color: #444;
  }
  @media print {
    .noprint {
      display: none;
    }
    .sheets {
      padding: 0;
    }
    .cards {
      grid-template-columns: 1fr 1fr;
      margin: 0;
    }
  }
</style>
