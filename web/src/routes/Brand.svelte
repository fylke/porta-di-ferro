<script lang="ts">
  import WeaponMark from './WeaponMark.svelte';
  import { t } from '../lib/i18n.svelte';
  import LangToggle from './LangToggle.svelte';

  /**
   * The graphical profile on one page (issue #89), at /brand.
   *
   * A profile that only exists as files in web/public/ is a profile nobody can check. The
   * decisions worth arguing with here are about size, not about taste -- whether the mark
   * survives a browser tab, whether four weapons stay four weapons at the size a list
   * renders them -- and neither question can be answered from a 512px PNG. So the page
   * shows every mark at the sizes it is actually used at, and the icon against light
   * browser chrome as well as dark, which is the one place a dark-only application gets
   * seen on white.
   */
  const weapons = [
    { key: 'longsword', name: t('Longsword'), note: t('Straight, double edged, blade to grip about three to one.') },
    { key: 'sabre', name: t('Sabre'), note: t('One edge and a curve. The curve is the whole difference, and it is enough.') },
    { key: 'rapier', name: t('Rapier'), note: t('Half the blade width of the longsword, and the ring of a swept hilt.') },
    { key: 'foam', name: t('Foam longsword'), note: t('The same sword, ending in a round instead of a point.') },
  ] as const;

  const iconSizes = [128, 64, 32, 16];
</script>

<main>
  <header>
    <img src="/icon.svg" alt="" width="44" height="44" />
    <div>
      <h1>{t('Graphical profile')}</h1>
      <p class="dim">{t('The mark, the weapons and the sizes they have to survive.')}</p>
    </div>
    <LangToggle />
  </header>

  <section>
    <h2>{t('The mark')}</h2>
    <p class="dim">
      {t('Porta di ferro is a guard, not a building, so the mark is the guard: a longsword held low with the point forward and down. The diagonal is what carries the small sizes — it is the one angle no interface furniture sits at, so the tab is findable in a row of them.')}
    </p>
    <div class="sizes">
      {#each iconSizes as s (s)}
        <figure>
          <img src="/icon.svg" alt={t('The mark at {n} pixels', { n: s })} width={s} height={s} />
          <figcaption>{s}px</figcaption>
        </figure>
      {/each}
      <figure class="on-light">
        <span>
          <img src="/icon.svg" alt="" width="32" height="32" />
          <img src="/icon.svg" alt="" width="16" height="16" />
        </span>
        <figcaption>{t('on light chrome')}</figcaption>
      </figure>
    </div>
  </section>

  <section>
    <h2>{t('The weapons')}</h2>
    <p class="dim">
      {t('One per discipline, drawn as one family: the same steel, the same blade, and all of them in the posture the application is named after. They take the colour and size of the text beside them, so there is no variant per screen.')}
    </p>
    <ul class="weapons">
      {#each weapons as w (w.key)}
        <li>
          <span class="big"><WeaponMark weapon={w.key} size="3.5rem" title={w.name} /></span>
          <span class="mid"><WeaponMark weapon={w.key} size="1.5rem" /></span>
          <span class="row">
            <span class="name">{w.name}</span>
            <span class="dim small">{w.note}</span>
            <span class="dim small inline">
              {t('In a list:')} <WeaponMark weapon={w.key} size="1.05em" /> {w.name}
            </span>
          </span>
        </li>
      {/each}
    </ul>
  </section>

  <section>
    <h2>{t('Why steel and nothing else')}</h2>
    <p class="dim">
      {t('Hue means identity in this application and never state: red is the red competitor, blue is the blue one, and amber belongs to neither because it is warnings. A mark that borrowed one of those would be the first thing to break the rule, on every screen at once. So the profile is steel: the two greys the interface already uses for ink.')}
    </p>
    <div class="swatches">
      <span class="swatch" style="background: #eef1f7; color: #12141a">#eef1f7</span>
      <span class="swatch" style="background: #98a0b4; color: #12141a">#98a0b4</span>
      <span class="swatch" style="background: #12141a; color: #eef1f7; outline: 1px solid #333949">#12141a</span>
    </div>
  </section>

  <section>
    <h2>{t('The files')}</h2>
    <ul class="files">
      <li><code>/icon.svg</code> <span class="dim">{t('the mark, and the favicon every current browser uses')}</span></li>
      <li><code>/icon-maskable.svg</code> <span class="dim">{t('the same mark inside the safe circle an Android launcher crops to')}</span></li>
      <li><code>/favicon.ico</code> <span class="dim">{t('16, 32 and 48, each drawn at its own size rather than resampled')}</span></li>
      <li><code>/icon-180.png</code> <span class="dim">{t('what Apple asks for, since it ignores the manifest')}</span></li>
      <li><code>/icon-192.png</code>, <code>/icon-512.png</code>, <code>/icon-maskable-512.png</code> <span class="dim">{t('the manifest')}</span></li>
    </ul>
    <p class="dim small">{t('The weapon marks are a component, not a file: they are inline SVG so they cost no request and inherit their colour.')}</p>
  </section>
</main>

<style>
  main {
    max-width: 58rem;
    margin: 0 auto;
    padding: 1.5rem 1.25rem 4rem;
    line-height: 1.55;
  }
  header {
    display: flex;
    align-items: center;
    gap: 0.9rem;
    margin-bottom: 1.6rem;
  }
  header div {
    flex: 1;
  }
  h1 {
    margin: 0;
    font-size: 1.4rem;
  }
  h2 {
    margin: 0 0 0.5rem;
    font-size: 0.78rem;
    font-weight: 800;
    letter-spacing: 0.12em;
    text-transform: uppercase;
    color: var(--ink-dim);
  }
  section {
    background: var(--panel);
    border-radius: var(--radius);
    padding: 1.1rem 1.25rem;
    margin-bottom: 1rem;
  }
  p {
    margin: 0 0 0.9rem;
  }
  .dim {
    color: var(--ink-dim);
  }
  .small {
    font-size: 0.85rem;
  }

  /* Shown at the sizes that decide it, actual size, side by side: a mark that only ever
     appears at 512 has not been reviewed. */
  .sizes {
    display: flex;
    align-items: flex-end;
    flex-wrap: wrap;
    gap: 1.6rem;
  }
  figure {
    margin: 0;
    text-align: center;
  }
  figcaption {
    margin-top: 0.4rem;
    font-size: 0.72rem;
    color: var(--ink-dim);
  }
  .on-light span {
    display: inline-flex;
    align-items: flex-end;
    gap: 0.6rem;
    background: #f4f5f7;
    padding: 0.7rem 0.9rem;
    border-radius: 8px;
  }

  .weapons {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    gap: 0.4rem;
  }
  .weapons li {
    display: flex;
    align-items: center;
    gap: 1.4rem;
    padding: 0.7rem 0;
    border-bottom: 1px solid var(--line);
  }
  .weapons li:last-child {
    border-bottom: none;
  }
  .big,
  .mid {
    display: inline-flex;
    justify-content: center;
    flex: none;
  }
  .big {
    width: 4rem;
  }
  .mid {
    width: 2rem;
  }
  .row {
    display: grid;
    gap: 0.15rem;
    min-width: 0;
  }
  .name {
    font-weight: 700;
  }
  .inline {
    color: var(--ink-dim);
  }

  .swatches {
    display: flex;
    flex-wrap: wrap;
    gap: 0.6rem;
  }
  .swatch {
    padding: 0.7rem 1rem;
    border-radius: 8px;
    font-size: 0.8rem;
    font-weight: 700;
    letter-spacing: 0.04em;
  }

  .files {
    list-style: none;
    margin: 0 0 0.6rem;
    padding: 0;
    display: grid;
    gap: 0.35rem;
    font-size: 0.9rem;
  }
  code {
    background: var(--panel-2);
    padding: 0.12em 0.4em;
    border-radius: 4px;
    font-size: 0.88em;
  }

  @media (max-width: 620px) {
    .weapons li {
      gap: 0.9rem;
    }
    .big {
      width: 3rem;
    }
    .mid {
      display: none;
    }
  }
</style>
