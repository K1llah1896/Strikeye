# Proposal: Demo Share Code Decoding

## Why

Players share matches as `CSGO-xxxxx-xxxxx-xxxxx-xxxxx-xxxxx` codes, and every
comparable tool (CS:DM, cs-demo-analyzer, cs-demo-downloader) accepts them.
Without decoding, Strikeye cannot recognize a code an analyst pastes, and any
future download flow would have to redo this primitive. The research report lists
share-code handling as part of the missing source-acquisition capability.

## What Changes

- New `internal/sharecode` package: Valve's 25-symbol/57-alphabet encoding over
  18 little-endian bytes (match id, reservation id, TV port).
- `Decode` tolerates the `CSGO-` prefix, dashes, and surrounding whitespace;
  `Encode`/`Canonical` produce the canonical grouped form.
- `strikeye-cli sharecode --code <code>` prints the identifiers as JSON.

## Impact

- Affected specs: `demo-sharecode` (new)
- Affected code: `internal/sharecode` (new), `cmd/strikeye-cli`
- No storage or schema changes.
