import { describe, expect, it } from 'vitest';
import { defaultOptions, optionsOf } from './options';
import type { Event } from './types';

describe('optionsOf', () => {
  it('has the defaults for a log with no options record', () => {
    expect(optionsOf([])).toEqual(defaultOptions());
  });

  it('lets the last record win and fills in what it left out', () => {
    const log: Event[] = [
      { seq: 1, type: 'options', elapsedMs: 0, options: { red: 'green', blue: 'yellow', swapDisplay: false } },
      { seq: 2, type: 'options', elapsedMs: 0, options: { red: '', blue: 'white', swapDisplay: true } },
    ];
    expect(optionsOf(log)).toEqual({ red: 'green', blue: 'white', swapDisplay: true });
  });

  it('falls back to the side colour for a name the palette does not know', () => {
    const log: Event[] = [
      { seq: 1, type: 'options', elapsedMs: 0, options: { red: 'mauve', blue: 'blue', swapDisplay: false } },
    ];
    expect(optionsOf(log).red).toBe('red');
  });
});
