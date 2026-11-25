# 导出支持 --format all

## 为什么

导出已经支持 csv/json/xlsx/html 四种格式与逗号组合，但最常用的一句"全都要"没有短写：`--format csv,json,xlsx,html` 写四次才等价。用 `all` 一词既省事，也让脚本里不必跟着格式列表扩张而改。

## 变什么

- `export.ParseFormats` 接受 `all`（等同于 `csv,json,xlsx,html`），`--format` 的分段解析里也接受 `all` 与其他词混用。
- 错误信息补上 `all`。
- 测试扩充 `TestParseFormats`（`all`、`csv,xlsx`）。

## 影响

- capability：`match-data-export`（MODIFIED「Per-match export」，同时补上 `_kills.csv` 与 `html`）。
- 纯 CLI/解析层改动，无数据结构变化。
