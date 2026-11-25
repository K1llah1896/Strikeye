# demo-download

## ADDED Requirements

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
