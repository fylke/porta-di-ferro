<script lang="ts">
  import { lang, setLang } from '../lib/i18n.svelte';

  /**
   * The language switch, the same two letters on every surface. Small, because it is
   * pressed once per device; never hidden, because a screen in the wrong language is a
   * screen somebody has to walk over to.
   */
  let { compact = false }: { compact?: boolean } = $props();
</script>

<span class="lang" class:compact role="group" aria-label="Language / Språk">
  <button class:on={lang.current === 'en'} aria-pressed={lang.current === 'en'} onclick={() => setLang('en')}>EN</button>
  <button class:on={lang.current === 'sv'} aria-pressed={lang.current === 'sv'} onclick={() => setLang('sv')}>SV</button>
</span>

<style>
  .lang {
    display: inline-flex;
    border: 1px solid var(--line);
    border-radius: 999px;
    overflow: hidden;
  }
  button {
    padding: 0.25rem 0.6rem;
    font-size: 0.75rem;
    font-weight: 800;
    letter-spacing: 0.06em;
    background: none;
    border: none;
    border-radius: 0;
    color: var(--ink-dim);
  }
  button.on {
    background: var(--ink);
    color: #0d0f14;
  }
  .compact button {
    padding: 0.15rem 0.45rem;
    font-size: 0.65rem;
  }
</style>
