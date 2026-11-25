# demo-download Specification

## Purpose
用用户自己的 Steam 网页登录态从 Valve GCPD 列出并下载官方比赛 demo，落到 watcher 的导入目录后自动进入解析队列，让「打完一局就能复盘」不必手动找文件。

## Requirements

### Requirement: GCPD match listing

系统 SHALL 能从 Steam GCPD 页面列出可下载的比赛：

- 请求 SHALL 携带用户提供的 Cookie 头与桌面 User-Agent；没有 Cookie 时 SHALL 直接报错而不是匿名请求。
- 解析 SHALL 从 `td.val_left` 单元格里取出 `/730/<matchID>_<outcomeID>.dem.bz2` 链接与 `YYYY-MM-DD HH:MM:SS GMT` 时间。
- 同一 matchID SHALL 只出现一次。
- 结果 SHALL 按时间倒序排列。
- 页面出现 `g_steamID = false` 或 HTTP 401/403 时 SHALL 返回"会话失效"错误。
- SHALL 支持通过 `continue_token` + `sessionid` + `ajax=1` 翻页，且最多抓 `MaxPages` 页。

#### Scenario: Parse two downloads and one expired row

- **WHEN** 页面里有两个 demo 链接与一行"Demo expired"
- **THEN** 返回两条比赛，matchID、outcomeID、时间与文件名都正确

#### Scenario: Duplicate match across cells

- **WHEN** 同一 matchID 在页面出现两次
- **THEN** 只返回一条

#### Scenario: Expired cookie

- **WHEN** 页面包含 `g_steamID = false`
- **THEN** 返回包含 "not signed in" 的错误

#### Scenario: Continue token

- **WHEN** 首页带 `g_sGcContinueToken` 且第二页还有一场
- **THEN** 两页结果合并为三条并按时间倒序

### Requirement: Demo archive download

`Client.Download` SHALL 把一场比赛的 demo 归档保存到目标目录：

- SHALL 流式写入 `<file>.part` 后再 rename。
- 目标文件已存在且非空时 SHALL 直接复用，不发起请求。
- 响应体为空时 SHALL 报错并删除临时文件。
- 非 200 状态 SHALL 返回带状态码的错误。

#### Scenario: Download then reuse

- **WHEN** 连续两次下载同一场
- **THEN** 只发出一次 HTTP 请求，文件内容为服务端字节

#### Scenario: Empty body

- **WHEN** 服务端返回 200 但 0 字节
- **THEN** 报错且目录里不留 `.part`

### Requirement: Download CLI

`strikeye-cli download` SHALL 接受 `--cookies`（文件或原始头）、`--tab`（逗号分隔多个）、`--limit`、`--out`，输出 `results[]`，每项含 `tab/listed/downloaded/reused/skipped/out_dir/matches`。

- `--out` 缺省 SHALL 为 `<DataRoot>/media/imports`，即 watcher 的导入目录。
- `--cookies` 指向存在的文件时 SHALL 用 `ReadCookieFile` 解析，支持 Netscape `cookies.txt`。
- `matches[].url` SHALL 输出本地落盘路径，便于后续定位。

#### Scenario: End to end

- **WHEN** 对包含一场比赛的页面执行 `download --limit 0`
- **THEN** `listed=1`、`downloaded=1`，归档出现在 `--out` 目录

#### Scenario: Missing cookie

- **WHEN** 没有传 `--cookies`
- **THEN** 命令以 "no steam cookie" 报错退出

### Requirement: In-app match history import

资料库 SHALL 提供应用内的 match history 下载入口：

- 面板 SHALL 接受粘贴的 Cookie 请求头与匹配类型选择（`gcpd.Tabs` 中的五种）。
- Cookie SHALL 保存到 `<DataRoot>/cache/gcpd_cookie.txt`，权限 0600，不得写入 `config.json`。
- 下载 SHALL 在后台执行，进行中 SHALL 显示「下载中...」并阻止重复触发。
- 下载目标 SHALL 是 `<DataRoot>/media/imports`，已存在的非空文件 SHALL 跳过并计入 `skipped`。
- 单场失败 SHALL 记录到失败列表并继续处理后续场次，不得中断整批。
- 完成后 SHALL 在状态栏显示下载/跳过/失败数量与目录，并触发一次资料库扫描。
- `STRIKEYE_GCPD_BASE` SHALL 覆盖 Steam BaseURL，与 CLI 行为一致。

#### Scenario: Paste cookie and download

- **WHEN** 用户粘贴有效 Cookie、选择 `matchhistorypremier` 并点击「下载并导入」
- **THEN** 新 demo 出现在 `media/imports`，状态栏显示下载数量，随后的扫描把文件登记进资料库

#### Scenario: Expired cookie

- **WHEN** Cookie 已失效
- **THEN** 状态栏显示「下载失败: gcpd: not signed in (cookie expired)」，不产生半个文件

#### Scenario: Already downloaded

- **WHEN** 同一场 demo 已在 `media/imports`
- **THEN** 该场被计入 `skipped`，不重复下载

### Requirement: Share code import

资料库 SHALL 支持用分享码导入比赛：

- 输入 SHALL 接受带或不带 `CSGO-` 前缀、带或不带分隔符的分享码。
- 解析出的 `match_id` SHALL 在 `gcpd.Tabs` 各标签页的 match history 中查找，命中即下载到 `<DataRoot>/media/imports`。
- 缺少 Cookie 时 SHALL 报错说明需要 Cookie；单个标签页列表失败 SHALL 跳过而不是中断。
- 目标文件已存在且非空时 SHALL 报"该场已导入"，不重复下载。
- 未找到时 SHALL 报错说明该场可能未在客户端下载过或 Cookie 已过期。
- 导入 SHALL 与批量下载共用进行中标志，成功后 SHALL 刷新资料库。

#### Scenario: Import a shared match

- **WHEN** 用户粘贴有效分享码且 Cookie 有效
- **THEN** 对应 demo 被下载到 `media/imports`，状态栏显示比赛 ID 与命中标签页

#### Scenario: Already imported

- **WHEN** 该 match id 的 demo 已存在于 `media/imports`
- **THEN** 状态栏显示「该场已导入」，不产生新文件

#### Scenario: Not in history

- **WHEN** 分享码有效但比赛不在 match history 中
- **THEN** 报错提示可能未在客户端下载过该场，或 Cookie 已过期
