<script lang="ts">
  import { t } from '../lib/i18n.svelte';

  /**
   * The weapon marks (issue #89).
   *
   * One per discipline the club runs, drawn as one family: the same steel, the same
   * blade construction, and all of them in the posture the application is named after --
   * point forward and down, the porta di ferro guard, which is also the app icon. A row
   * of them reads as a set rather than as four clip-art swords.
   *
   * Filled with `currentColor` and sized in `em`, so a mark takes the colour and size of
   * the text beside it and needs no variant per surface. That matters here: the same
   * mark has to sit in an organizer's 0.9rem list and on a hall display seen from thirty
   * metres, and the palette it lands on is the caller's business (web/src/app.css).
   *
   * What tells them apart is silhouette, not detail: the sabre by its curve, the rapier
   * by a blade half the width of the longsword's, the foam trainer by a blade that ends
   * in a round instead of a point. Every one of those survives being 18px wide in a list,
   * which is where these are actually read, and nothing here relies on a line thinner
   * than the blade it belongs to.
   */
  let {
    weapon,
    size = '1.25em',
    title,
  }: {
    weapon: 'longsword' | 'sabre' | 'rapier' | 'foam';
    /** Any CSS length. Defaults to a little over the line it sits on. */
    size?: string;
    /** An accessible name. Omitted makes the mark decorative, for a label that says it. */
    title?: string;
  } = $props();

  const names: Record<string, string> = {
    longsword: t('Longsword'),
    sabre: t('Sabre'),
    rapier: t('Rapier'),
    foam: t('Foam longsword'),
  };
  const label = $derived(title ?? names[weapon]);
</script>

<svg
  class="mark"
  viewBox="0 0 48 48"
  width={size}
  height={size}
  fill="currentColor"
  role={title === undefined ? 'presentation' : 'img'}
  aria-label={title === undefined ? undefined : label}
>
  {#if title !== undefined}<title>{label}</title>{/if}
  <g transform="rotate(-35 24 24)">
    {#if weapon === 'longsword'}
      <!-- Straight, double edged, long grip, disc pommel: blade to grip about three to
           one, which is the proportion that makes it a longsword and not a dagger. -->
      <path
        d="M24 1.5a3.2 3.2 0 1 1 0 6.4 3.2 3.2 0 0 1 0-6.4Z
           M21.9 7.4h4.2v7h-4.2Z
           M16.4 13.8h15.2a2 2 0 0 1 0 4H16.4a2 2 0 0 1 0-4Z
           M21.1 17.8h5.8l-1.2 22.6L24 45.5l-1.7-5.1Z"
      />
    {:else if weapon === 'sabre'}
      <!-- One edge, and a curve that carries the whole silhouette: the only thing that
           separates this from the longsword, and enough on its own. A knuckle bow was
           drawn here and taken out again -- at 18px it was a loop hanging off the grip,
           and nobody was reading it as a hilt. The tip is clipped rather than pointed,
           which is what a sabre's is. -->
      <path
        d="M24 2.8a3 3 0 1 1 0 6 3 3 0 0 1 0-6Z
           M22.1 8.2h3.8v6h-3.8Z
           M17.4 13.4h13.2a2 2 0 0 1 0 4H17.4a2 2 0 0 1 0-4Z
           M21.6 17.6h4.9c.2 8.4 2.4 15.4 6.7 21.4l-3.2 2.6c-5.4-7-8.4-15-8.4-24Z"
      />
    {:else if weapon === 'rapier'}
      <!-- Half the blade width of the longsword, and a ring at the hilt: thin and long is
           the whole read. -->
      <path
        d="M24 1.6a2.7 2.7 0 1 1 0 5.4 2.7 2.7 0 0 1 0-5.4Z
           M22.4 6.6h3.2v6.2h-3.2Z
           M14.6 12.2h18.8a1.7 1.7 0 0 1 0 3.4H14.6a1.7 1.7 0 0 1 0-3.4Z
           M22.7 15.4h2.6l-.5 27.4-.8 3.6-.8-3.6Z"
      />
      <!-- The ring of a swept hilt. Small enough to stay a ring: at four units across it
           was reading as a magnifying glass hung off the blade. -->
      <path
        d="M24 14.2a3.9 3.9 0 1 1 0 7.8 3.9 3.9 0 0 1 0-7.8Zm0 2.2a1.7 1.7 0 1 0 0 3.4 1.7 1.7 0 0 0 0-3.4Z"
      />
    {:else}
      <!-- The foam trainer: the longsword's hilt and the longsword's taper, ending in a
           round instead of a point, which is the whole reason it exists. Drawn without
           the taper first, and a straight-sided blade on that hilt reads as a club. -->
      <path
        d="M24 1.5a3.2 3.2 0 1 1 0 6.4 3.2 3.2 0 0 1 0-6.4Z
           M21.9 7.4h4.2v7h-4.2Z
           M16.4 13.8h15.2a2 2 0 0 1 0 4H16.4a2 2 0 0 1 0-4Z
           M21.3 17.8h5.4l-.6 22.4a2.1 2.1 0 0 1-4.2 0Z"
      />
    {/if}
  </g>
</svg>

<style>
  /* Sits on the text baseline like a letter would, rather than on the line box, so a mark
     beside a discipline's name does not push the row taller. */
  .mark {
    display: inline-block;
    vertical-align: -0.2em;
    flex: none;
  }
</style>
