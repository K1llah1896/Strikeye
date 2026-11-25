# Tasks

## 1. Head-to-head

- [x] 1.1 `internal/h2h`: `Report`/`MapRecord`/`MatchRecord`、`Build(ctx, db, teamA, teamB)`
- [x] 1.2 方向归一化、分图聚合、最近交手排序、队名校验
- [x] 1.3 `cmd/strikeye-cli/h2h.go`: `runHeadToHead`（json/csv）

## 2. Player trend

- [x] 2.1 `internal/profile`: `TrendPoint`、`Trend(ctx, db, steamID)`（对手、胜负、日期排序）
- [x] 2.2 `runTrends`（json/csv）

## 3. Verification

- [x] 3.1 `internal/h2h/h2h_test.go`: 方向归一化、分图、未解析忽略、参数校验
- [x] 3.2 `internal/profile/profile_test.go`: 时间顺序、对手与胜负、未知选手、空 steamid
- [x] 3.3 `cmd/strikeye-cli/tendency_test.go`: 缺参报错
- [x] 3.4 真实数据：FURIA vs Falcons 2 场 2-0（29-20 回合），de_ancient 16-14、de_dust2 13-6；KSCERATO 趋势 101.6 ADR → 113.6 ADR
