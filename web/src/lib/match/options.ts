import type { Event, Options } from './types';

/**
 * The colours a competitor can be shown in. Names, not values: the CSS palette in app.css
 * defines a tint and a bright for each, and the displays and the score keeper view look
 * them up by name, so the log stays readable and the palette can be tuned in one place.
 */
export const COLOURS = ['red', 'blue', 'green', 'yellow', 'orange', 'purple', 'white'] as const;
export type Colour = (typeof COLOURS)[number];

export function isColour(name: string): name is Colour {
  return (COLOURS as readonly string[]).includes(name);
}

/** The red side in red, the blue side in blue, red on the left of the displays. */
export function defaultOptions(): Options {
  return { red: 'red', blue: 'blue', swapDisplay: false };
}

/**
 * Reads the presentation options out of a log: the last options record wins, and a log
 * with none has the defaults. Mirrors OptionsOf in internal/match/options.go.
 *
 * Separate from replay on purpose. State is what the vectors pin, field for field, in
 * both engines; colours are not a scoring matter and must not be able to fail a vector.
 */
export function optionsOf(events: Event[]): Options {
  const out = defaultOptions();
  for (const e of events) {
    if (e.type !== 'options' || !e.options) continue;
    if (e.options.red) out.red = e.options.red;
    if (e.options.blue) out.blue = e.options.blue;
    out.swapDisplay = e.options.swapDisplay;
  }
  // A name the palette does not know falls back to the side's own colour, so a hand-edited
  // log cannot produce an unstyled panel.
  if (!isColour(out.red)) out.red = 'red';
  if (!isColour(out.blue)) out.blue = 'blue';
  return out;
}
