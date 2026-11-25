# Spec Delta

## Purpose

解析与生成 CS2/CS:GO 比赛分享码（CSGO-xxxxx-xxxxx-xxxxx-xxxxx-xxxxx），供后续 Demo 下载与比赛关联使用，并为命令行提供查询入口。

## ADDED Requirements

### Requirement: Share code decoding
The system SHALL decode a 25-symbol share code into match id, reservation id, and TV port.

#### Scenario: Canonical code
- **WHEN** the operator decodes `CSGO-L9spZ-ihuov-cyhtE-kxbqa-FkBAA`
- **THEN** the result is match id 3400360672356205056, reservation id 3400367402569957763, port 9725

#### Scenario: Loose input
- **WHEN** the input omits the `CSGO-` prefix, omits dashes, or has surrounding whitespace
- **THEN** it decodes to the same identifiers

#### Scenario: Invalid symbols
- **WHEN** the code contains a symbol outside the alphabet or has the wrong length
- **THEN** decoding fails with an error and no partial match is returned

### Requirement: Share code encoding
The system SHALL encode identifiers back into the canonical grouped share code.

#### Scenario: Round trip
- **WHEN** identifiers decoded from a valid code are encoded again
- **THEN** the canonical grouped form matches the original code exactly

### Requirement: Share code CLI
The system SHALL expose decoding through `strikeye-cli sharecode --code`.

#### Scenario: JSON output
- **WHEN** the command runs with a valid code
- **THEN** it prints `match_id`, `reservation_id`, `tv_port`, and the canonical `share_code`

#### Scenario: Missing code
- **WHEN** `--code` is empty
- **THEN** the command fails with a `--code is required` error
