# Graphical profile

The mark, the weapon marks and the icon files (issue #89). The page at `/brand` in the
running application shows all of it at the sizes it is used at, which is the only way to
review this kind of work: a mark that has only been looked at large has not been looked
at.

## The mark

Porta di ferro is a guard, not a building, so the mark is the guard — a longsword held
low with the point forward and down.

Two earlier drawings put the sword inside a literal gate, once as an arch and once as a
lintel on two jambs. Both failed at the size that decides an icon. A frame with a sword
through it reads at 16px as a window with a cross in it, which is the wrong thing to say
twice over, and the torii the second one made is the wrong continent for an Italian
longsword club.

What makes the final one work small:

- **The diagonal.** No interface furniture sits at 35°, so the tab is findable in a row
  of them, and the blade stays off the pixel grid that flattened the upright versions
  into a plus sign.
- **Blade to grip about three to one.** A shorter blade on the same hilt is a dagger, and
  the first pass drew one.
- **Nothing thinner than the blade.** No hairlines, no strokes that survive at 512 and
  vanish at 16.

## Colour

Steel only. No red, no blue, no amber.

Hue means identity in this application and never state (`web/src/app.css`): red is the
red competitor, blue is the blue one, and amber belongs to neither because it is
warnings. A brand mark that borrowed one of them would be the first thing to break that
rule, on every screen at once.

| Token | Value | Use |
| --- | --- | --- |
| ink | `#eef1f7` | the blade |
| ink-dim | `#98a0b4` | secondary steel |
| bg | `#12141a` | the square behind it |

## The weapon marks

`web/src/routes/WeaponMark.svelte`, one per discipline the club runs: `longsword`,
`sabre`, `rapier`, `foam`. Inline SVG filled with `currentColor` and sized in `em`, so a
mark takes the colour and size of the text beside it and costs no request. There is no
variant per screen, which matters when the same mark has to sit in an organizer's 0.9rem
list and on a hall display seen from thirty metres.

All four are drawn in the mark's own posture, so a row of them reads as a set rather than
as four pieces of clip art. What separates them is silhouette, not detail, because they
are read at 18px:

- **Longsword** — straight, double edged, tapering to a point.
- **Sabre** — one edge and a curve, with a clipped tip. A knuckle bow was drawn and taken
  out again: at 18px it was a loop hanging off the grip and nobody read it as a hilt. The
  curve is the whole difference and it is enough.
- **Rapier** — half the longsword's blade width, plus the ring of a swept hilt. The ring
  was twice this size at first and read as a magnifying glass.
- **Foam longsword** — the same sword ending in a round instead of a point. It needs the
  longsword's taper: drawn straight-sided it reads as a club.

## The files

All in `web/public/`, which Vite copies to `dist/` and the Go binary embeds.

| File | Why it exists |
| --- | --- |
| `icon.svg` | the mark, and the favicon every current browser uses |
| `icon-maskable.svg` | the same mark inside the safe circle an Android launcher crops to: full bleed, no corner radius of its own, artwork at 62% |
| `favicon.ico` | the file a browser asks for without being told to. Without it the request falls through to `index.html`, because the server answers unknown extensionless-or-not paths with the app |
| `icon-16/32/48.png` | the ICO's three entries, and each is drawn at its own size rather than resampled from one render |
| `icon-180.png` | `apple-touch-icon`. Apple ignores the manifest |
| `icon-192.png`, `icon-512.png` | the manifest, `purpose: any` |
| `icon-maskable-512.png` | the manifest, `purpose: maskable` |

### Regenerating the rasters

The SVGs are the source; the PNGs and the ICO are built from them. Nothing in CI does
this, because it is done when the drawing changes and not otherwise.

```sh
cd web/public
for s in 16 32 48 180 192 512; do
  inkscape -e "icon-$s.png" -w "$s" -h "$s" icon.svg
done
inkscape -e icon-maskable-512.png -w 512 -h 512 icon-maskable.svg
```

Then pack the ICO. Pillow's own ICO writer resamples a single source, so the 16px entry
would be a blurred 48px render rather than the one that was drawn at 16; an ICO entry may
be a PNG as it stands, so the three files go in as they are:

```python
import struct

images = [(s, open(f'icon-{s}.png', 'rb').read()) for s in (16, 32, 48)]
out = struct.pack('<HHH', 0, 1, len(images))
offset = 6 + 16 * len(images)
for size, data in images:
    out += struct.pack('<BBBBHHII', size, size, 0, 0, 1, 32, len(data), offset)
    offset += len(data)
out += b''.join(data for _, data in images)
open('favicon.ico', 'wb').write(out)
```
