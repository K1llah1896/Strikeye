# Proposal: 比赛数据导出与跨比赛聚合

## Why

报告指出同类工具（cs-demo-analyzer 的 CSV/JSON 导出、cs-demo-manager 的 XLSX/JSON/SQL 导出）允许用户把分析结果带出工具做二次分析；本项目目前只能 UI 内查看，无法导出，也没有跨比赛聚合视图。

## What Changes

- 新增 `match-data-export` 能力：将单场比赛的选手、回合、事件、阵营数据导出为 CSV（多文件）与 JSON（单文件）。
- 新增按 SteamID 的跨比赛聚合：逐场明细 + 合计（场次/击杀/死亡/助攻/ADR/Rating/KAST/HS%）。
- `strikeye-cli export` 子命令：`--demo-id` 或 `--steamid`，`--out` 目录，`--format csv|json|both`。
- 导出文件写入 `<DataRoot>/media/exports`（可由 `--out` 覆盖），文件名带 demo ID 或 SteamID。

## Impact

- Affected specs: `match-data-export`（ADDED）
- Affected code: `internal/export/`（新）、`cmd/strikeye-cli/main.go`
