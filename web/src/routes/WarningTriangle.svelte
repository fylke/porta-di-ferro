<script lang="ts">
  /**
   * One warning, as a triangle with an exclamation mark in it.
   *
   * It replaces a bare U+25B2. That character reads as a bullet or a sort arrow at
   * scoreboard sizes, and what it has to say is a count: the second warning costs a point
   * and the third ends the match, so the score keeper needs to see how many there are and
   * that they are warnings, not decoration.
   *
   * Amber comes from `currentColor`, so it stays whatever the surrounding text is -- the
   * one colour in the language that belongs to neither competitor (design §4).
   */
  let { label = 'Warning' }: { label?: string } = $props();
</script>

<svg class="warning-triangle" viewBox="0 0 24 22" role="img" aria-label={label}>
  <!-- Stroked as well as filled, with a round join: the corners of a printed or projected
       triangle at this size are the first thing to look ragged, and rounding them is what
       makes it read as a sign rather than an artefact. -->
  <path
    d="M12 2.6 21.3 18.9a1.6 1.6 0 0 1-1.4 2.4H4.1a1.6 1.6 0 0 1-1.4-2.4Z"
    fill="currentColor"
    stroke="currentColor"
    stroke-width="2.4"
    stroke-linejoin="round"
  />
  <!-- The mark is punched out dark rather than drawn light, so it survives on any amber. -->
  <rect x="10.9" y="8.4" width="2.2" height="6.4" rx="1.1" fill="#20180a" />
  <circle cx="12" cy="17.7" r="1.3" fill="#20180a" />
</svg>

<style>
  .warning-triangle {
    height: 1em;
    width: auto;
    display: block;
    flex: none;
  }
</style>
