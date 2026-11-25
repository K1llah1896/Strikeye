## Why

报告里「官方/平台 Demo 下载」是唯一还剩的 P0 数据缺口。CS:DM 与 cs-demo-downloader 的做法一致：用 Steam 网页登录态访问 GCPD（`steamcommunity.com/my/gcpd/730`）拿 demo 链接，再直接下载 `replayN.valve.net/730/<match>_<outcome>.dem.bz2`。

demo 文件本身不需要鉴权，只有列表页需要 Cookie——所以这是一个可以完整实现的客户端，缺的只是用户自己的登录态（任何工具都无法代替）。

## What Changes

- 新增 `internal/gcpd`：`Client.List(tab)` 解析 GCPD 页面（含 `continue_token` 翻页）、`Client.Download(match, dir)` 保存 `.dem.bz2`、`ParsePage` 纯函数、`ReadCookieFile` 支持原始 Cookie 头与 Netscape `cookies.txt`。
- 新增 CLI `download --cookies <file|header> --tab <tab> --limit <n> --out <dir>`；默认输出到 `<DataRoot>/media/imports`，也就是 watcher 的导入目录，落盘后自动进入解析队列。
- 会话失效检测：页面出现 `g_steamID = false` 或 HTTP 401/403 时给出"cookie 过期"的明确错误。
- 已存在的同名归档直接复用，不重复下载。

## Impact

- 新增：`internal/gcpd`、`cmd/strikeye-cli/download.go`。
- 复用 `demo-watcher` 的归档导入（bz2 → 解压 → 解析），无需改动 watcher。
- 需要用户在运行时提供 Steam Cookie；程序不保存、不打印 Cookie。
