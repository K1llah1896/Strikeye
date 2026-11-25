# Design: Duel Timing Metrics

## Definition

TTK = `(kill tick − first damage tick by the killer on that victim) / tickRate`.
The clock starts at the first positive health damage in the victim's current
life; a different attacker resets the clock to their own first hit, and any gap
longer than 3 seconds starts a new engagement. Only kills whose damage and kill
happen in the same round and whose damage is enemy damage count.

Self-damage, team damage, and zero-damage events are ignored, matching the
existing capped-damage rules.

## Why 3 seconds

Trade windows use 5 seconds elsewhere. Three seconds covers a realistic
burst/chip sequence without letting pre-round utility damage inflate the metric;
5 s produced 15-second averages on real demos because it chained unrelated
engagements.

## Storage

Two columns rather than recomputing on read: parsing already holds both event
streams in memory, and the UI reads `match_players` without touching replay data.
Average is rounded to 0.1 ms at write time.

## Limitations

Demos where the killing bullet produces no separate `PlayerHurt` (common for
one-taps) contribute no sample, so the average reflects multi-hit duels. TTD
and visibility-aware timing remain open; they require prepared map geometry.
