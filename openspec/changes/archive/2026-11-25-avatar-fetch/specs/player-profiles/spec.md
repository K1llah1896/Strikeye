# Spec Delta

## Purpose

为选手档案补上头像：从 Steam 公开资料页抓取并缓存，分析页在有缓存时显示头像。

## ADDED Requirements

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
