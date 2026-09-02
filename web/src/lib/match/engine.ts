import type { Competitor, Event, State } from './types';
import { emptyState, type Ruleset } from './ruleset';

/**
 * Derives a match's state from its log. Mirrors Replay in internal/match/engine.go.
 *
 * Two passes. The first collects the sequence numbers voided by undo records; the second
 * applies everything that survives. That is why a correction is an append rather than a
 * mutation, and why full history editing (Milestone 2) is a UI change rather than a data
 * migration: the engine already skips any voided record, not merely the last one.
 */
export function replay(r: Ruleset, events: Event[]): State {
  const undone = new Set<number>();
  for (const e of events) {
    if (e.type === 'undo' && e.undo) undone.add(e.undo.seq);
  }

  const s = emptyState();

  // The stack of exchanges that actually took effect. An exchange voided by an undo never
  // reaches it, and neither does one ignored because the match was already over by rule --
  // so the top is exactly what the Undo control acts on, and never an exchange that
  // changed nothing.
  const applied: number[] = [];

  for (const e of events) {
    if (e.seq > s.lastSeq) s.lastSeq = e.seq;
    if (undone.has(e.seq)) continue;
    switch (e.type) {
      case 'exchange':
        if (applyExchange(r, s, e)) applied.push(e.seq);
        break;
      case 'timer':
        applyTimer(s, e);
        break;
      case 'undo':
        // The void itself was recorded in the first pass and the record it names is
        // skipped above. All that is left is to drop the dialog it had raised.
        s.pending = 'none';
        break;
      case 'end':
        applyEnd(r, s, e);
        break;
    }
  }
  if (!s.ended && applied.length > 0) s.undoableSeq = applied[applied.length - 1];
  return s;
}

function applyExchange(r: Ruleset, s: State, e: Event): boolean {
  if (s.ended || !e.exchange) return false;
  // After either cap the match is over by rule, and the only legal next records are the
  // end itself or an undo of the exchange that tripped it. Ignoring further exchanges
  // keeps a stray write from moving a score off a recorded 0-8. The time condition is
  // different: play may legitimately continue.
  if (s.pending === 'point_cap' || s.pending === 'penalty_cap') return false;

  s.elapsedMs = e.elapsedMs;
  s.exchanges++;

  // Differential scoring: the difference between the two assessments is awarded, not both
  // values. A 2 against a 1 gives the winner 1 and the other nothing; a 2 against a 2
  // gives nobody anything. Afterblows and doubles need no special handling -- they net
  // out. Confirming with nothing selected nets zero, which is a real no-score exchange
  // and is counted above.
  const net = e.exchange.red.value - e.exchange.blue.value;
  if (net > 0) s.red.score += net;
  else if (net < 0) s.blue.score -= net;

  // Penalties apply after the points, so a deduction lands on the score this exchange
  // produced.
  const redLost = applyPenalty(r, s.red, e.exchange.red.penalty);
  const blueLost = applyPenalty(r, s.blue, e.exchange.blue.penalty);

  // A penalty loss is recorded 0-8 against the warned competitor, whatever the running
  // score was, and earns them no match points.
  if (redLost && blueLost) {
    // Only reachable through Milestone 2 escalation. Nobody wins, nobody scores.
    s.red.score = 0;
    s.blue.score = 0;
    s.pending = 'penalty_cap';
    return true;
  }
  if (redLost) {
    s.red.score = 0;
    s.blue.score = r.forfeitScore;
    s.pending = 'penalty_cap';
    return true;
  }
  if (blueLost) {
    s.red.score = r.forfeitScore;
    s.blue.score = 0;
    s.pending = 'penalty_cap';
    return true;
  }

  // The cap takes precedence over the time condition when one confirmation trips both.
  if (s.red.score >= r.pointCap || s.blue.score >= r.pointCap) {
    s.pending = 'point_cap';
    return true;
  }
  if (e.elapsedMs >= r.matchTimeMs) {
    s.pending = 'final_exchange';
    return true;
  }
  s.pending = 'none';
  return true;
}

/**
 * Advances a competitor's penalty level and reports whether that took them to the losing
 * level. The deduction fires when the level crosses the deduction level, so an immediate
 * escalation from 0 straight to 2 still costs the point.
 */
function applyPenalty(r: Ruleset, c: Competitor, levels: number): boolean {
  if (levels <= 0) return false;
  const before = c.penalty;
  c.penalty += levels;
  if (before < r.penaltyDeduction && c.penalty >= r.penaltyDeduction) {
    c.score--;
    // Floored at zero, matching the Go engine. design §5 does not say what happens to a
    // competitor who has no points, and a negative would feed two ranking indices.
    if (c.score < 0) c.score = 0;
  }
  return c.penalty >= r.penaltyLoss;
}

function applyTimer(s: State, e: Event): void {
  if (s.ended || !e.timer) return;
  s.elapsedMs = e.elapsedMs;
  if (e.timer.action === 'start' || e.timer.action === 'resume') s.running = true;
  else if (e.timer.action === 'stop') s.running = false;
}

function applyEnd(r: Ruleset, s: State, e: Event): void {
  if (!e.end) return;
  s.elapsedMs = e.elapsedMs;
  s.ended = true;
  s.running = false;
  s.pending = 'none';
  s.endReason = e.end.reason;

  if (e.end.reason === 'forfeit') {
    // A conceded match is recorded 0-8 and earns the forfeiter no match points.
    if (e.end.forfeiter === 'red') {
      s.red.score = 0;
      s.blue.score = r.forfeitScore;
      s.winner = 'blue';
    } else {
      s.blue.score = 0;
      s.red.score = r.forfeitScore;
      s.winner = 'red';
    }
    s.noMatchPoints = true;
    return;
  }

  s.noMatchPoints = e.end.reason === 'penalty';
  if (s.red.score > s.blue.score) s.winner = 'red';
  else if (s.blue.score > s.red.score) s.winner = 'blue';
  else s.winner = '';
}
