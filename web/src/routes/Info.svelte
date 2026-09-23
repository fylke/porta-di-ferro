<script lang="ts">
  import { onMount } from 'svelte';
  import { api, type Address } from '../api';
  import { Live } from '../lib/live.svelte';
  import { t, lang } from '../lib/i18n.svelte';
  import LangToggle from './LangToggle.svelte';
  import Schedule from './Schedule.svelte';

  /**
   * The sheet that goes on the door (issue #98).
   *
   * The most restricted view in the application, and the only one whose reader has no
   * network yet — that is what it is for. So everything is on it twice: a code to scan
   * and the same thing in text, because a camera that will not focus in a dim entrance
   * is the normal case rather than the edge one.
   *
   * It is a printout first. The PDF beside it is the same content laid out for A4, and
   * the page itself prints directly from a browser as well.
   */
  const live = new Live();
  let addresses = $state<Address[]>([]);

  onMount(() => {
    live.start();
    api
      .addresses()
      .then((a) => (addresses = a))
      .catch(() => {
        // No list means no address to advertise, which the page says below.
      });
    return () => live.stop();
  });

  const snapshot = $derived(live.snapshot);
  const event = $derived(snapshot?.tournament.event ?? {});
  const wifi = $derived(event.wifi ?? {});

  // Substituted at build time: false in the real application, and everything behind it
  // goes with it.
  const demo = import.meta.env.VITE_DEMO === 'true';

  // The address a phone should open. Never this page's own origin when that is localhost:
  // the whole point is an address another device can reach.
  const port = $derived(window.location.port ? `:${window.location.port}` : '');
  const here = $derived(addresses.find((a) => a.ip === window.location.hostname));
  const chosen = $derived(here ?? addresses[0] ?? null);
  // The demo has no LAN to enumerate, and its landing page really is this origin -- so
  // there the page's own address is the honest one to print rather than nothing.
  const landing = $derived(
    chosen ? `http://${chosen.ip}${port}/` : demo ? window.location.origin + '/' : '',
  );

  /**
   * What a phone's camera reads as "join this network". Built here as well as in the PDF
   * because the two are printed from different places and must not disagree; the escaping
   * is the format's, and a venue password with a semicolon in it is exactly the case that
   * breaks when it is skipped.
   */
  const wifiPayload = $derived.by(() => {
    const ssid = (wifi.ssid ?? '').trim();
    if (!ssid) return '';
    const security = wifi.security || 'WPA';
    const esc = (v: string) => v.replace(/([\\;,:"])/g, '\\$1');
    let out = `WIFI:T:${security};S:${esc(ssid)};`;
    if (security !== 'nopass') out += `P:${esc(wifi.password ?? '')};`;
    if (wifi.hidden) out += 'H:true;';
    return out + ';';
  });

  /**
   * The server renders these; the demo has no server, and an <img> load does not go
   * through the shim that stands in for one, so there the module hands back a data URL.
   * VITE_DEMO is a build-time constant, so the branch is gone from the real bundle.
   */
  const qr = (payload: string) =>
    demo && window.portaQR ? window.portaQR(payload) : `/api/qr.png?url=${encodeURIComponent(payload)}`;
  const pdfURL = $derived(
    `/api/info.pdf?lang=${lang.current}&url=${encodeURIComponent(landing)}`,
  );
</script>

<main>
  <header class="screen-only">
    <a class="back" href="/admin">{t('Admin')}</a>
    <a class="pdf" href={pdfURL}>{t('Export PDF')}</a>
    <LangToggle />
  </header>

  <h1>{snapshot?.instance.name || 'Porta di Ferro'}</h1>
  <p class="strap">{t('Results and schedule on your phone')}</p>

  <div class="sheet">
    <section class="welcome">
      {#if event.welcome}
        <p>{event.welcome}</p>
      {:else}
        <p class="dim">{t('No welcome message has been written yet. The admin view is where it goes.')}</p>
      {/if}
    </section>

    <section class="wayin">
      {#if wifiPayload}
        <div class="step">
          <h2>{t('Join the wifi')}</h2>
          <div class="withcode">
            <img src={qr(wifiPayload)} alt={t('Wifi code for {ssid}', { ssid: wifi.ssid ?? '' })} width="160" height="160" />
            <dl>
              <dt>{t('Network')}</dt>
              <dd class="strong">{wifi.ssid}</dd>
              {#if wifi.security !== 'nopass'}
                <dt>{t('Password')}</dt>
                <dd class="mono">{wifi.password}</dd>
              {/if}
            </dl>
          </div>
        </div>
      {/if}

      <div class="step">
        <h2>{wifiPayload ? t('Then open') : t('Open this on your phone')}</h2>
        {#if landing}
          <div class="withcode">
            <img src={qr(landing)} alt={t('Code for {url}', { url: landing })} width="160" height="160" />
            <dl>
              <dt>{t('Scan, or type it in')}</dt>
              <dd class="mono url">{landing}</dd>
            </dl>
          </div>
        {:else}
          <p class="dim">{t('This PC is not on a network another device could reach, so there is no address to print. Join it to the venue wifi and reload.')}</p>
        {/if}
      </div>
    </section>

    <section class="programme">
      <h2>{t('Programme')}</h2>
      <Schedule items={event.schedule ?? []} />
    </section>
  </div>
</main>

<style>
  main {
    max-width: 62rem;
    margin: 0 auto;
    padding: 1rem 1rem 3rem;
  }
  header {
    display: flex;
    align-items: center;
    gap: 1rem;
    margin-bottom: 1rem;
  }
  .back {
    flex: 1;
    color: var(--ink-dim);
    text-decoration: none;
    font-size: 0.9rem;
  }
  .pdf {
    color: var(--ink);
    font-size: 0.9rem;
  }
  h1 {
    margin: 0;
    font-size: clamp(1.6rem, 6vw, 2.6rem);
    overflow-wrap: anywhere;
  }
  .strap {
    margin: 0.2rem 0 1.2rem;
    color: var(--ink-dim);
    font-size: clamp(0.95rem, 2.4vw, 1.1rem);
  }
  h2 {
    margin: 0 0 0.6rem;
    font-size: 0.78rem;
    font-weight: 800;
    letter-spacing: 0.12em;
    text-transform: uppercase;
    color: var(--ink-dim);
  }
  section {
    background: var(--panel);
    border-radius: var(--radius);
    padding: 1.1rem 1.2rem;
  }
  .dim {
    color: var(--ink-dim);
  }

  /* One column on a phone; the sketch's arrangement from tablet portrait up, with the
     way in beside the welcome and the programme across the bottom. */
  .sheet {
    display: grid;
    gap: 0.8rem;
    grid-template-columns: minmax(0, 1fr);
  }
  .sheet > * {
    min-width: 0;
  }
  .welcome p {
    margin: 0;
    line-height: 1.6;
    font-size: clamp(1rem, 2.6vw, 1.15rem);
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }

  .wayin {
    display: grid;
    gap: 1.2rem;
  }
  .withcode {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 1rem;
  }
  .withcode img {
    width: clamp(7rem, 30vw, 10rem);
    height: auto;
    border-radius: 8px;
    /* The codes are dark-on-light and stay that way: a scanner wants the contrast the
       format specifies, not the page's palette. */
    background: #fff;
    padding: 0.4rem;
  }
  dl {
    margin: 0;
    min-width: 0;
  }
  dt {
    font-size: 0.72rem;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--ink-dim);
    margin-top: 0.5rem;
  }
  dt:first-child {
    margin-top: 0;
  }
  dd {
    margin: 0.1rem 0 0;
    font-size: 1.05rem;
    overflow-wrap: anywhere;
  }
  .strong {
    font-weight: 700;
  }
  .url {
    font-size: clamp(0.9rem, 2.6vw, 1.15rem);
  }

  @media (min-width: 46rem) {
    .sheet {
      grid-template-columns: minmax(0, 1.4fr) minmax(17rem, 1fr);
    }
    .welcome {
      grid-column: 1;
      grid-row: 1;
    }
    .wayin {
      grid-column: 2;
      grid-row: 1;
    }
    .programme {
      grid-column: 1 / -1;
      grid-row: 2;
    }
  }

  /* Printing the page itself, for an organizer who would rather hit Ctrl-P than download
     the PDF. Same content, on paper, in ink that does not cost anything. */
  @media print {
    .screen-only {
      display: none;
    }
    main {
      max-width: none;
      padding: 0;
    }
    section {
      background: none;
      border: 1px solid #bbb;
    }
    :global(html),
    :global(body) {
      background: #fff;
      color: #000;
    }
    .dim,
    dt {
      color: #555;
    }
  }
</style>
