# Spec Delta

## Purpose

为已导入的 Demo 生成在 CS2 客户端内直接播放所需的启动链接与可选跳转 cfg，替代侵入式游戏内插件方案，并为命令行提供输出入口。

## ADDED Requirements

### Requirement: CS2 launch hand-off
The system SHALL produce a Steam launch URL that opens CS2 with `+playdemo` for a stored demo.

#### Scenario: URL contains one playdemo flag
- **WHEN** a launch URL is generated for any demo path
- **THEN** the URL starts with `steam://rungameid/730//+playdemo` and contains exactly one `+`

#### Scenario: Path characters are escaped
- **WHEN** the demo path contains spaces, backslashes, or `+`
- **THEN** those characters are percent-encoded (`%20`, `%5C`, `%2B`)

#### Scenario: File name is CS2 friendly
- **WHEN** the demo is stored at `/data/media/demos/m1.dem`
- **THEN** the reported file name is `m1.dem`

### Requirement: Optional tick seek config
The system SHALL emit a `demo_goto <tick>` config body when a positive start tick is requested.

#### Scenario: Tick provided
- **WHEN** the operator requests tick 12345
- **THEN** the output config contains `demo_goto 12345`

#### Scenario: No tick
- **WHEN** the start tick is zero or omitted
- **THEN** no config is emitted

### Requirement: Playback CLI
The system SHALL expose the hand-off through `strikeye-cli play --demo-id`.

#### Scenario: JSON output
- **WHEN** the command runs for an imported demo
- **THEN** it prints `demo_id`, `demo_path`, `file_name`, `start_tick`, `launch_url`, and `config` when present

#### Scenario: Missing demo id
- **WHEN** `--demo-id` is zero
- **THEN** the command fails with a `--demo-id is required` error
