# Spec Delta

## Purpose

为比赛与选手数据提供可分享的导出格式，供人工复盘、外部工具与二次分析使用。

## ADDED Requirements

### Requirement: HTML match report
The system SHALL export a match as one self-contained HTML document.

#### Scenario: Report content
- **WHEN** a match is exported with the `html` format
- **THEN** `match_<id>.html` contains the team names, score, map, date, per-side scoreboards, and the round list

#### Scenario: No external assets
- **WHEN** the report is generated
- **THEN** it references no scripts, images, stylesheets, or remote URLs

#### Scenario: Escaped dynamic text
- **WHEN** a player name or team name contains HTML metacharacters
- **THEN** the characters are escaped in the output

#### Scenario: Combined formats
- **WHEN** the operator passes `csv,html` or `json,html`
- **THEN** each requested format is written and unsupported names still fail with an error
