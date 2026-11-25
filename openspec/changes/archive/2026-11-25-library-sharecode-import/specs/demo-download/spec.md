# demo-download

## ADDED Requirements

### Requirement: Share code import

资料库 SHALL 支持用分享码导入比赛：

- 输入 SHALL 接受带或不带 `CSGO-` 前缀、带或不带分隔符的分享码。
- 解析出的 `match_id` SHALL 在 `gcpd.Tabs` 各标签页的 match history 中查找，命中即下载到 `<DataRoot>/media/imports`。
- 缺少 Cookie 时 SHALL 报错说明需要 Cookie；单个标签页列表失败 SHALL 跳过而不是中断。
- 目标文件已存在且非空时 SHALL 报"该场已导入"，不重复下载。
- 未找到时 SHALL 报错说明该场可能未在客户端下载过或 Cookie 已过期。
- 导入 SHALL 与批量下载共用进行中标志，成功后 SHALL 刷新资料库。

#### Scenario: Import a shared match

- **WHEN** 用户粘贴有效分享码且 Cookie 有效
- **THEN** 对应 demo 被下载到 `media/imports`，状态栏显示比赛 ID 与命中标签页

#### Scenario: Already imported

- **WHEN** 该 match id 的 demo 已存在于 `media/imports`
- **THEN** 状态栏显示「该场已导入」，不产生新文件

#### Scenario: Not in history

- **WHEN** 分享码有效但比赛不在 match history 中
- **THEN** 报错提示可能未在客户端下载过该场，或 Cookie 已过期
