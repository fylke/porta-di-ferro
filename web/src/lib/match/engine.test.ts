import { describe, expect, it } from 'vitest';
import { readFileSync, readdirSync } from 'node:fs';
import { join } from 'node:path';
import fc from 'fast-check';
import { MSL, replay, type Event, type State } from './index';

/**
 * The shared corpus. These are the same files internal/match runs, so a vector that
 * passes there and fails here fails the build (docs/tech-stack.md §4).
 *
 * The corpus is an executable statement of the ruleset, which is why it is worth more
 * than the duplication costs: when rules become data-driven in Milestone 3 it is already
 * the specification a ruleset definition has to satisfy.
 */
const vectorDir = join(import.meta.dirname, '../../../../testdata/vectors');

interface Vector {
  name: string;
  description: string;
  events: Event[];
  expect: State;
}

function loadVectors(): Vector[] {
  return readdirSync(vectorDir)
    .filter((f) => f.endsWith('.json'))
    .sort()
    .map((f) => JSON.parse(readFileSync(join(vectorDir, f), 'utf8')) as Vector);
}

describe('shared vectors', () => {
  const vectors = loadVectors();

  it('covers the ruleset', () => {
    expect(vectors.length).toBeGreaterThanOrEqual(20);
  });

  for (const v of vectors) {
    it(`${v.name}: ${v.description}`, () => {
      expect(replay(MSL, v.events)).toEqual(v.expect);
    });
  }

  it('replays deterministically', () => {
    for (const v of vectors) {
      expect(replay(MSL, v.events)).toEqual(replay(MSL, v.events));
    }
  });
});

/**
 * A plausible match log: a start, then a run of exchanges inside the ruleset's range. No
 * undo records, so the invariants below can be stated over a monotonic history. Mirrors
 * genLog in internal/match/property_test.go.
 */
const arbLog = fc
  .array(
    fc.record({
      gap: fc.integer({ min: 1000, max: 20000 }),
      redValue: fc.integer({ min: 0, max: 2 }),
      redPenalty: fc.integer({ min: 0, max: 1 }),
      blueValue: fc.integer({ min: 0, max: 2 }),
      bluePenalty: fc.integer({ min: 0, max: 1 }),
    }),
    { maxLength: 30 },
  )
  .map((rows) => {
    const events: Event[] = [
      { seq: 1, type: 'timer', elapsedMs: 0, timer: { action: 'start' } },
    ];
    let elapsed = 0;
    rows.forEach((row, i) => {
      elapsed += row.gap;
      events.push({
        seq: i + 2,
        type: 'exchange',
        elapsedMs: elapsed,
        exchange: {
          red: { value: row.redValue, penalty: row.redPenalty },
          blue: { value: row.blueValue, penalty: row.bluePenalty },
        },
      });
    });
    return events;
  });

describe('properties', () => {
  it('no exchange moves a score by more than the ruleset allows', () => {
    fc.assert(
      fc.property(arbLog, (events) => {
        let prev = replay(MSL, events.slice(0, 1));
        for (let i = 2; i <= events.length; i++) {
          const got = replay(MSL, events.slice(0, i));
          if (got.pending === 'penalty_cap' && prev.pending !== 'penalty_cap') {
            prev = got;
            continue;
          }
          for (const side of ['red', 'blue'] as const) {
            const delta = got[side].score - prev[side].score;
            expect(delta).toBeLessThanOrEqual(MSL.maxValue);
            expect(delta).toBeGreaterThanOrEqual(-1);
          }
          prev = got;
        }
      }),
    );
  });

  it('penalty escalation is monotonic', () => {
    fc.assert(
      fc.property(arbLog, (events) => {
        let prev = replay(MSL, events.slice(0, 1));
        for (let i = 2; i <= events.length; i++) {
          const got = replay(MSL, events.slice(0, i));
          expect(got.red.penalty).toBeGreaterThanOrEqual(prev.red.penalty);
          expect(got.blue.penalty).toBeGreaterThanOrEqual(prev.blue.penalty);
          prev = got;
        }
      }),
    );
  });

  it('replay depends only on the log', () => {
    fc.assert(
      fc.property(arbLog, (events) => {
        expect(replay(MSL, events)).toEqual(replay(MSL, events));
      }),
    );
  });

  it('a score never goes negative', () => {
    fc.assert(
      fc.property(arbLog, (events) => {
        const got = replay(MSL, events);
        expect(got.red.score).toBeGreaterThanOrEqual(0);
        expect(got.blue.score).toBeGreaterThanOrEqual(0);
      }),
    );
  });
});
