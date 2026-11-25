# config-and-paths Specification

## Purpose

Manages Strikeye's persistent application configuration, local DataRoot directory hierarchy, and automatic probing of external CS2, Steam, HLAE, and FFmpeg toolchains without any OBS dependencies.

## Requirements

### Requirement: DataRoot layout initialization
The system SHALL initialize the required DataRoot subdirectories (`backup/db_snapshots`, `backup/cs2_user_configs`, `cache/replay_32hz`, `cache/radar_cards/cards`, `cache/radar_cards/animations`, `cache/radar_cards/portraits`, `cache/radar_cards/team_logos`, `media/pov_rounds`, `media/highlights`, `media/exports`) before reading or writing runtime data.

#### Scenario: First launch on clean DataRoot
- **WHEN** the application initializes against an empty DataRoot directory
- **THEN** all required DataRoot subdirectories exist and are writable without manual creation

### Requirement: Atomic configuration load and save
The system SHALL load `config.json` from DataRoot, apply safe defaults for missing fields, and write configuration updates atomically so partial writes never corrupt existing settings.

#### Scenario: Save configuration atomically
- **WHEN** updated settings are saved to `config.json`
- **THEN** the new configuration is persisted via temporary file replacement without leaving truncated JSON on disk

### Requirement: Corrupt configuration quarantine
The system SHALL preserve and quarantine an unparseable `config.json` with a timestamped backup suffix and regenerate a clean default configuration instead of failing startup.

#### Scenario: Malformed JSON in config file
- **WHEN** `config.json` contains invalid JSON syntax at startup
- **THEN** the corrupt file is renamed to a quarantine backup and a valid default `config.json` is created

### Requirement: External toolchain probing without OBS
The system SHALL probe configured paths, environment overrides, Steam library manifests, and standard installation locations to report availability of Steam, CS2 (`cs2.exe`), HLAE (`HLAE.exe` and `AfxHookSource2.dll`), and FFmpeg/FFprobe, and MUST NOT probe or store OBS configuration.

#### Scenario: Toolchain diagnostic check
- **WHEN** environment probing runs with valid CS2, HLAE, and FFmpeg paths configured
- **THEN** diagnostic status reports each component as available with its resolved executable path and reports no OBS fields or dependencies

#### Scenario: Missing external binary
- **WHEN** HLAE or FFmpeg is absent from configured and standard locations
- **THEN** probing completes without crashing and marks only the missing component as unavailable with a diagnostic reason

### Requirement: Theme configuration persistence and normalization
The system SHALL persist a `theme` configuration block in `config.json` containing the selected preset name, custom accent color hex, custom background image path, background overlay opacity, and card opacity, and SHALL normalize missing or out-of-range values to safe defaults during load.

#### Scenario: Missing or invalid theme fields on load
- **WHEN** `config.json` omits the `theme` block or contains an unrecognized preset name, invalid accent hex, or out-of-range opacity
- **THEN** configuration normalization fills the default esports dark preset, `#F59E0B` accent color, and clamped opacity values without failing startup

#### Scenario: Saving updated theme settings
- **WHEN** the user saves theme changes from the UI
- **THEN** `config.json` is updated atomically with the new `theme` block and remains valid JSON on disk

### Requirement: Probe discovers bundled tools

工具链探测 SHALL 在 DataRoot 的 `tools/` 目录中发现随包解压的二进制，使设置页与渲染链路无需用户填写路径：

- `ProbeOptions.DataRoot` 非空时 SHALL 检查 `<DataRoot>/tools/<tool>/` 下的候选路径：`<exe>`、`bin/<exe>`、`*/<exe>`、`*/bin/<exe>`、`*/*/bin/<exe>`；
- 候选顺序 SHALL 位于系统 PATH 之前，使包内版本优先；
- 已显式配置且文件存在的路径 SHALL 优先于所有候选；
- 探测 SHALL 使用 glob 而非目录遍历，且不得因目录不存在而报错。

#### Scenario: Staged ffmpeg wins over PATH

- **WHEN** `<DataRoot>/tools/ffmpeg/ffmpeg-9.1-essentials_build/bin/ffmpeg.exe` 存在
- **THEN** 探测结果 `ffmpeg.path` 指向该文件，即使系统 PATH 里也有 ffmpeg

#### Scenario: Staged HLAE brings its hook

- **WHEN** `<DataRoot>/tools/hlae/HLAE.exe` 与 `x64/AfxHookSource2.dll` 存在
- **THEN** `hlae` 与 `afx_hook_source2` 都报告可用

#### Scenario: Configured path still wins

- **WHEN** 用户显式配置了 `paths.ffmpeg_exe` 且该文件存在
- **THEN** 探测返回该路径，不切换为内置版本

#### Scenario: Missing tools directory is not an error

- **WHEN** DataRoot 下没有 `tools/` 目录
- **THEN** 探测正常返回，其余字段与未加该能力前一致
