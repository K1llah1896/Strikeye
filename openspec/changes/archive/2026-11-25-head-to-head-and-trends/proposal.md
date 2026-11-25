# Proposal: 对阵矩阵与选手趋势

## Why

`tendency-report` 让教练看清一支队怎么打，但赛前还有两个问题它回答不了：

1. **两队历史交手**：谁在哪些图上占优、最近一次是什么比分。分析师目前只能用 `sql` 手写 join。
2. **选手的走势**：`profile` 把跨场数据压成一个聚合值，看不出"这个人在最近三场是上升还是下滑"。

两者都是已有数据的重新组织，不需要新表、不需要新依赖。

## What Changes

- `internal/h2h`：按两个队名归一化取数（同一场比赛队伍可能在 `team_a` 或 `team_b`），输出总体战绩、分图战绩、最近交手列表。
- `profile.Trend`：按日期升序输出选手逐场数据（对手、地图、K/D/A、ADR、KAST、HS%、Rating、胜负）。
- CLI：`h2h --team-a --team-b`、`trends --steamid`，均支持 `--format json|csv`。

## Impact

- Affected specs: `tendency-report`（新增两个 requirement）
- Affected code: `internal/h2h`、`internal/profile`、`cmd/strikeye-cli/h2h.go`、`main.go`
