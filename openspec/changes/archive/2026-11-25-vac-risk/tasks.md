# Tasks

## 1. Storage

- [x] 1.1 迁移 v6 `player_bans` + 索引
- [x] 1.2 `PlayerBansRepo`：Upsert/Get/List/BannedFor

## 2. Client and service

- [x] 2.1 `internal/vac.Client.Fetch`（key、超时、1MB 上限、错误截断）
- [x] 2.2 `Ban.Risk()/RiskLabel()`
- [x] 2.3 `Service.Lookup`（TTL 24h、`--refresh`、无 key 只读缓存、`hit/refreshed/skipped/miss`）

## 3. Surfaces

- [x] 3.1 CLI `bans --demo-id|--steamid --refresh --api-key`
- [x] 3.2 资料库卡片「封禁 N」徽章
- [x] 3.3 `profile` 报告附带 `bans`

## 4. Verification

- [x] 4.1 httptest 单测：解析/无 key/403/缓存命中/过期刷新/无 key 降级
- [x] 4.2 真实运行 `bans --data-root dist`（无 key → skipped=10）
- [x] 4.3 真实运行带无效 key（Steam 403 错误透传）
- [x] 4.4 `go test ./...`、`openspec validate --all --strict`
