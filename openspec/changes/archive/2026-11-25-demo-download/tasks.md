# Tasks

## 1. Client

- [x] 1.1 `ParsePage`（URL + 时间窗口、去重）
- [x] 1.2 `List`（Cookie/UA、continue_token 翻页、会话失效检测）
- [x] 1.3 `Download`（流式、`.part`、复用、空体保护）
- [x] 1.4 `ReadCookieFile`（原始头 + Netscape）

## 2. CLI

- [x] 2.1 `download --cookies --tab --limit --out`
- [x] 2.2 默认输出到 `media/imports` 供 watcher 接管
- [x] 2.3 JSON 报告 `results[]`

## 3. Verification

- [x] 3.1 单测：解析/去重/翻页/过期会话/403/下载复用/空体/Cookie 解析
- [x] 3.2 CLI 端到端（httptest GCPD + demo 归档落盘）
- [x] 3.3 `go test ./...`、`openspec validate demo-download --strict`
