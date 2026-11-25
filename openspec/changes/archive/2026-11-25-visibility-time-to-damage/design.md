# Design: Visibility-Aware Time to Damage

## Sampling

`FrameDone` already samples at 32 Hz for the replay cache. The same hook records
`visSample{Tick, map[steamID]{x,y,z+64}}` for alive players, trimming anything
older than 5 s. Eye height is +64 units on both ends of the trace.

## Measurement

On `PlayerHurt` with positive damage, distinct attacker/victim, and opposite
teams, the parser records at most one sample per `(round, attacker, victim)`
pair. It walks the ring backwards from the damage tick:

- stop at a sample older than 5 s, a missing player, or an occluded sight line;
- the run length is `damageTick - oldestVisibleTick`, converted to ms.

This is "continuous visibility before the hit", so a victim who stepped out of
cover 200 ms before being hit reports ~200 ms, and a wallbang reports nothing.

## Storage

Migration v5 adds `ttd_avg_ms REAL` and `ttd_samples INTEGER` to
`match_players`. `ComputeMetrics` distributes samples to the all-round and
CT/T accumulators through the existing `addForRound` helper, so side splits stay
consistent with ADR/KAST.

## Cost

One BVH trace per 32 Hz sample per duel: at most ~160 traces per engagement,
each a few microseconds on a 270k-node BVH. Parsing the two-map fixture grew
from ~20 s to ~27 s with geometry present.

## Verification

Synthetic wall geometry unit-tests the run search (blocked vs clear, window
cutoff); metrics tests cover aggregation; storage tests cover the round trip;
re-parsing both fixtures produced plausible TTDs (KSCERATO 456 ms over 37
samples, NiKo 572 ms) while a data root without geometry still parsed with zero
TTD samples.
