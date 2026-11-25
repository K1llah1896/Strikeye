# Proposal: Open Demo in CS2

## Why

Strikeye exports video, but analysts still want to jump into the game's own demo
player at a specific round. The research report lists `open-in-cs2` as the
zero-invasion alternative to CS:DM's in-game plugin: hand off the demo path to
CS2 instead of building a plugin.

## What Changes

- New `internal/playback` package: `steam://rungameid/730//+playdemo%20...`
  launch URL, optional `demo_goto <tick>` cfg body, and a CS2-safe file name.
- `strikeye-cli play --demo-id [--tick]` prints demo path, launch URL, and cfg.
- No files are written or moved; the operator copies the demo where CS2 expects
  it and pastes the URL.

## Impact

- Affected specs: `open-demo-in-cs2` (new)
- Affected code: `internal/playback` (new), `cmd/strikeye-cli`
- Read-only over storage.
