# Design: Demo Share Code Decoding

## Format

The code is a base-57 number rendered least-significant digit first, using the
alphabet `ABCDEFGHJKLMNOPQRSTUVWXYZabcdefhijkmnopqrstuvwxyz23456789` (57 symbols,
case-sensitive, no `I`, `l`, `o`, `0`, `1`). The 25 digits decode to 18 bytes,
read little-endian as `uint64 matchID | uint64 reservationID | uint16 tvPort`.

## Implementation

`math/big` carries the accumulator; no overflow, no fixed-width tricks. Decoding
walks the symbols right-to-left because encoding emits the remainder first.
Malformed input (wrong length, unknown symbol, >18-byte payload) returns an
error instead of a zero match.

## Why not a downloader yet

Downloading a match by code needs a Steam login (cookie or auth code) and team
platform APIs. That cannot be verified offline, so this change ships the pure
decoding primitive that the downloader will sit on top of.

## Verification

Encoding and decoding are both checked against six real Valve share codes
(cross-checked with `akiver/cs-demo-analyzer`), plus loose-input and
rejection cases.
