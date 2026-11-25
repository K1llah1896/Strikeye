# Spec Delta

## Purpose

扩展 32Hz 回放缓存，使 2D 回放能够呈现弹道、死亡与 C4 事件。

## ADDED Requirements

### Requirement: Combat event replay chunk and version compatibility
The replay collector SHALL append a combat chunk (shots, deaths, bomb actions) to each round payload and SHALL bump the `CS2RPL01` format version while remaining able to read older caches.

#### Scenario: v2 cache carries combat events
- **WHEN** a demo is parsed by the current parser version
- **THEN** each round payload includes shot, death, and bomb event chunks alongside poses and utility effects, and the header records format version 2

#### Scenario: v1 cache remains readable
- **WHEN** the reader opens a `CS2RPL01` file written with format version 1
- **THEN** it returns the legacy rounds with empty combat event slices instead of failing

#### Scenario: Unknown newer version is rejected
- **WHEN** the reader encounters a format version greater than the supported version
- **THEN** it returns `ErrUnsupportedReplayVer`

#### Scenario: Corrupt combat chunk is rejected
- **WHEN** a v2 payload ends before a declared combat event record is complete
- **THEN** decoding returns `io.ErrUnexpectedEOF`
