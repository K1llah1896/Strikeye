# player-profiles Specification

## Purpose
为每位选手建立按 SteamID64 持久化的档案（显示名覆盖、侦察备注、标签、头像），并在 CLI 与比赛库中读取、编辑与检索这些档案；头像在打开比赛时自动补齐，用户无需点击下载。

## Requirements

### Requirement: Persistent player profiles
The system SHALL store one profile per SteamID64 with an optional display-name override, a free-form note, and a normalized tag list, and SHALL preserve parsed match_players rows unchanged.

#### Scenario: Tags are normalized
- **WHEN** a profile is saved with tags `["Star", " star ", "", "awp"]`
- **THEN** the stored list contains exactly `Star` and `awp`

#### Scenario: Missing profile is not an error
- **WHEN** the UI or CLI looks up a SteamID64 with no stored profile
- **THEN** it proceeds with an empty profile and `has_profile` false

#### Scenario: Migration upgrades existing databases
- **WHEN** an existing v2 database is opened with the new migrations
- **THEN** `player_profiles` is created, existing demo/player rows are untouched, and a pre-migration snapshot is written

### Requirement: Player dossier CLI
The system SHALL expose a player dossier through `strikeye-cli profile` that combines cross-match aggregates with the stored profile.

#### Scenario: Inspect a player
- **WHEN** the operator runs `strikeye-cli profile --steamid <id64>`
- **THEN** the command prints aggregate totals plus the profile and whether it exists

#### Scenario: Save profile fields
- **WHEN** the operator passes `--name`, `--note`, or `--tags`
- **THEN** the command persists only the supplied fields and prints the saved profile

#### Scenario: Player without matches
- **WHEN** the SteamID64 appears in no parsed match
- **THEN** the inspect command exits non-zero with a "no matches" error

### Requirement: Player tags in Match Library
The system SHALL surface stored player tags on match cards and include them in keyword filtering.

#### Scenario: Cards badge tags
- **WHEN** a demo's players have stored tags and the library refreshes
- **THEN** the card shows up to three tag pills

#### Scenario: Keyword matches a tag
- **WHEN** the operator searches a keyword equal to a stored tag
- **THEN** the library lists demos whose players carry that tag

### Requirement: Avatar fetch and cache

系统 SHALL 按 SteamID64 抓取并缓存选手头像：

- SHALL 从 `https://steamcommunity.com/profiles/<steamid>/?xml=1` 解析 `avatarFull`，再下载该图片。
- 下载 SHALL 限制资料页 512KB、图片 2MB，并原子写入 `<DataRoot>/cache/radar_cards/portraits/<steamid>.png|jpg`（`.part` → rename）。
- 缓存 SHALL 在 30 天内复用，`--refresh` SHALL 强制重新抓取。
- 非 17 位数字的 SteamID64 SHALL 返回 `ErrInvalidSteamID`；资料页无 `avatarFull` 或 404 SHALL 返回 `ErrNotFound`。
- 批量抓取 SHALL 逐个记录成功/缓存/失败，单项失败不影响其它项。

#### Scenario: Cached avatar is reused

- **WHEN** 头像已缓存且修改时间在 30 天内
- **THEN** 不再发起网络请求，返回已有路径

#### Scenario: Private profile

- **WHEN** 资料页存在但没有 `avatarFull`（私密资料）
- **THEN** 返回 `ErrNotFound` 且不写文件

#### Scenario: JPEG keeps its extension

- **WHEN** `avatarFull` 指向 `.jpg`
- **THEN** 缓存文件为 `<steamid>.jpg`

### Requirement: Avatar rendering in analysis

系统 SHALL 在分析页选手表渲染缓存头像：

- 选手有可解码的缓存头像时 SHALL 显示 18px 圆形缩略图。
- 无缓存或解码失败 SHALL 回退到阵营色圆点，且不影响表格其余内容。
- 渲染路径 SHALL NOT 发起网络请求。
- 头像 SHALL 在切换比赛时随选手列表重新加载。

#### Scenario: Corrupt cache file

- **WHEN** 缓存文件存在但不是有效图片
- **THEN** 该选手回退到圆点，其它选手头像正常显示

### Requirement: Avatar CLI

系统 SHALL 提供 `strikeye-cli avatars`：

- SHALL 接受 `--steamid <id64,...>` 或 `--demo-id <id>`（取该场全部选手）。
- SHALL 输出 JSON，含 `dir`、`fetched[]`、`cached[]`、`failed[{steamid,error}]`。
- 两者都缺 SHALL 返回错误。

#### Scenario: Demo roster

- **WHEN** 以 `--demo-id 1` 运行且该场有 10 名选手
- **THEN** 对每个 SteamID64 各抓取或复用一次，重复 SteamID 只处理一次

### Requirement: Manual avatar refresh

分析页 SHALL 支持手动刷新选手头像：

- 「全场选手数据」标题行 SHALL 提供「刷新头像」按钮；进行中 SHALL 显示徽章并阻止重复触发。
- 刷新 SHALL 忽略 30 天 TTL 重取全队头像，完成后重新解码并更新表格位图。
- 完成后 SHALL 显示「头像：新下载 N，已缓存 M」。
- `avatars.Client` 的零值（未设置 `HTTP`）SHALL 回退到 `http.DefaultClient`，不得 panic。

#### Scenario: Refresh roster avatars

- **WHEN** 用户点击「刷新头像」
- **THEN** 位图按新下载结果更新，标题行显示新下载与已缓存数量

#### Scenario: Zero client

- **WHEN** 调用方用只设置 `ProfileURL` 的 `avatars.Client{}` 发起请求
- **THEN** 请求使用默认 HTTP 客户端并返回网络错误，而不是崩溃

### Requirement: Roster portraits fetch automatically

分析页打开一场比赛时 MUST 自动补齐缺失的选手头像，用户无需点击任何按钮。

#### Scenario: Missing portraits are fetched on demo open

- **WHEN** 用户打开一场比赛，且部分选手在 `<DataRoot>/cache/radar_cards/portraits` 下没有头像文件
- **THEN** 视图在后台按 SteamID 抓取缺失头像（30 天 TTL），写盘后重新解码位图并重绘选手表
- **AND** 已有缓存的选手不产生任何网络请求

#### Scenario: Automatic fetch can be disabled or faked

- **WHEN** `View.avatarAutoFetch` 为 false，或注入了 `View.avatarClient`
- **THEN** 自动抓取不启动，或改用注入的客户端
- **AND** 渲染测试因此在无网络环境下保持确定性
