# Spec Delta

## Purpose

在既有 CSV / JSON / HTML 之外提供 XLSX 工作簿，让分析师直接在表格软件里透视比赛数据。

## ADDED Requirements

### Requirement: XLSX workbook export

系统 SHALL 支持 `--format xlsx` 导出：

- 比赛包 SHALL 生成 `match_<demo_id>.xlsx`，包含 `players`、`rounds`、`events`、`sides` 四张工作表。
- 选手聚合 SHALL 生成 `player_<steam_id64>_matches.xlsx`。
- 每张工作表 SHALL 带一行加粗表头，列定义与同内容 CSV 完全一致。
- 数值字段 SHALL 写成数字单元格而不是文本，以便表格软件直接求和、求平均。
- 工作簿 SHALL 是合法的 OOXML 包，至少包含 `[Content_Types].xml`、`_rels/.rels`、`xl/workbook.xml`、`xl/_rels/workbook.xml.rels`、`xl/styles.xml` 与每张表的 `xl/worksheets/sheetN.xml`。

#### Scenario: Export a match to XLSX

- **WHEN** 用户执行 `strikeye-cli export --demo-id 2 --format xlsx`
- **THEN** 输出目录中出现 `match_2.xlsx`，四张工作表可按名字访问，`kills` 等列是数字

#### Scenario: Numeric columns stay numeric

- **WHEN** 某选手 ADR 为 88.5
- **THEN** 单元格值是数字 88.5，而不是字符串 "88.5"

#### Scenario: CSV and XLSX agree on columns

- **WHEN** 同一比赛分别以 CSV 与 XLSX 导出
- **THEN** `players` 表的列名与顺序和 `match_<id>_players.csv` 表头一致
