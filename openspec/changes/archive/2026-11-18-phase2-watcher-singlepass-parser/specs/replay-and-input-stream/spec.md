# Spec Delta

## Purpose

Encodes and reads the CS2RPL01 binary cache containing 32Hz 2D replay frames, grenade/smoke/inferno geometry, and embedded raw-tick player input telemetry without fabricating mouse trajectories.

## ADDED Requirements

### Requirement: 32Hz replay and grenade geometry capture
During the single-pass parse, the collector SHALL sample living player poses and state at `stride = max(1, round(tickRate / 32.0))` ticks alongside grenade projectile paths, smoke detonation/dissipation windows, and active inferno 2D convex hulls.

#### Scenario: 32Hz frames and utility geometry persisted to CS2RPL01
- **WHEN** a 64-tick demo completes single-pass parsing
- **THEN** `<DataRoot>/cache/replay_32hz/<sha256>.rpl` is written atomically with `CS2RPL01` magic, round index offsets, 32Hz player frames, and grenade/smoke/inferno chunks, and `demos.replay_cache_path` is updated

### Requirement: Embedded raw-tick player input stream and capability reporting
The input collector SHALL extract button edge transitions (`W, A, S, D, Jump, Crouch, Walk, Reload, Fire, Scope, Use, Inspect, Scoreboard`) from `CSVCMsg_UserCmd` with entity `m_nButtons` fallback, embed them as a raw-tick chunk inside `CS2RPL01`, and set `demos.input_data_capability` to `full_usercmd`, `entity_buttons_only`, or `none`.

#### Scenario: UserCmd stripped in platform demo
- **WHEN** a demo lacks `CSVCMsg_UserCmd` messages but exposes entity `m_nButtons`
- **THEN** `demos.input_data_capability` is recorded as `entity_buttons_only` and button transitions are still embedded in `CS2RPL01`

#### Scenario: Input telemetry absent without fabricated mouse paths
- **WHEN** neither `CSVCMsg_UserCmd` nor `m_nButtons` is available in the demo
- **THEN** `demos.input_data_capability` is recorded as `none` and the system MUST NOT synthesize physical mouse trajectories from view angles or movement

### Requirement: Indexed CS2RPL01 round-level random access
The `CS2RPL01` reader SHALL validate file headers and section offsets so callers can load metadata, a single round's 32Hz replay/grenade frames, or a tick range's input transitions without decoding the entire file.

#### Scenario: Single-round replay and input slice read
- **WHEN** a reader requests round 5 from an `.rpl` file
- **THEN** only the header, round index, and round 5 chunks are decoded and returned
