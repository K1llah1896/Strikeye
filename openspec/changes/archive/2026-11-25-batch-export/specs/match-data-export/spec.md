# match-data-export

## ADDED Requirements

### Requirement: Batch export across matches

`strikeye-cli export --all` SHALL export every parsed match in one run:

- 每场比赛 SHALL 写入独立目录 `<out>/demo_<id>/`，文件名沿用单场导出的 `match_<id>_*` 规则。
- SHALL 支持 `--team`（大小写不敏感，命中 A 或 B 任一侧）、`--map`（大小写不敏感）与 `--limit`（按 id 升序最多 N 场）。
- 只导出 `parse_status = parsed` 的比赛。
- 批末 SHALL 写 `<out>/export_index.json`，包含格式化后的过滤条件、`exported`/`failed` 计数与每场的 `demo_id`、队伍、地图、目录、文件列表、错误。
- 单场失败 SHALL 记入索引并继续；全部失败时命令 SHALL 返回错误。
- `--all` SHALL 与 `--demo-id`、`--steamid` 互斥，同时给出时 SHALL 报错。

#### Scenario: Export every parsed match
- **WHEN** 库中有 2 场 parsed 与 1 场 pending，运行 `export --all --format all`
- **THEN** 生成 `demo_<id1>/` 与 `demo_<id2>/` 各 4 种格式文件，pending 场次被跳过，`export_index.json` 中 `exported=2`

#### Scenario: Filter by team
- **WHEN** 运行 `export --all --team navi` 且只有一场包含 NAVI
- **THEN** 只导出那一场，索引中 `filter` 含 `team=navi`

#### Scenario: Empty selection
- **WHEN** 过滤条件没有任何比赛命中
- **THEN** 命令返回 `no matches matched filter …` 错误且不写索引
