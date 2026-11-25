# Spec Delta

## Purpose

在资料库页把镜像的 GSI 状态渲染成一行直播条，让用户在不看终端的情况下知道当前比赛的比分、回合与存活人数。

## ADDED Requirements

### Requirement: Library live banner

系统 SHALL 在资料库页顶部渲染一行 GSI 直播状态：

- 仅当 `cache/gsi_latest.json` 存在且修改时间在 15 秒内时才渲染。
- 内容 SHALL 含地图、回合号、CT-T 比分、阶段、存活人数（`CTvT`）。
- 炸弹状态 SHALL 在 `planted`/`defusing`/`exploded` 时追加，`carried`/`dropped`/空值 SHALL 省略。
- 文件缺失、过期或 JSON 无法解析时 SHALL 不渲染任何行，且不影响资料库其它内容。
- 读取频率 SHALL 不超过每 2 秒一次。
- 镜像 SHALL 由 `strikeye-cli gsi` 原子写入 `<DataRoot>/cache/gsi_latest.json`，UI 侧只读，不得写入或修改该文件。

#### Scenario: Fresh mirror renders banner

- **WHEN** 镜像存在、修改时间为 1 秒前，载荷含 `map=de_ancient`、`round=9`、比分 5-4、`bomb=planted`、2 名存活 CT 与 1 名存活 T
- **THEN** 横幅文本为 `de_ancient · 回合 9 · 5-4 · planted · 炸弹 planted · 存活 2v1`

#### Scenario: Stale mirror hides banner

- **WHEN** 镜像修改时间为 30 秒前且新鲜度窗口为 15 秒
- **THEN** 判定为过期，资料库不渲染横幅

#### Scenario: Carried bomb is omitted

- **WHEN** 载荷 `bomb=carried`
- **THEN** 横幅不出现炸弹片段
