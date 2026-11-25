# gsi-live

## ADDED Requirements

### Requirement: GSI payload ingestion

系统 SHALL 解析 Counter-Strike 2 Game State Integration 载荷：

- 数字字段 SHALL 同时接受 JSON 数字与带引号的数字；无法识别的值 SHALL 视为 0 而不是报错。
- 解析结果 SHALL 含地图、阶段、回合号、双方比分、回合阶段、炸弹状态、剩余回合时间与选手列表。
- 选手列表 SHALL 合并 `player` 与 `allplayers`，以 `steamid` 去重，`player` 优先。
- 每位选手 SHALL 含阵营（CT/T）、血量、护甲、头盔、金钱、击杀/助攻/死亡/MVP/评分、武器列表与存活标志（`health > 0`）。

#### Scenario: Quoted numbers

- **WHEN** `round.phase_ends_in` 为字符串 "21.4"
- **THEN** 解析为 21.4，且不因该字段失败

#### Scenario: Merge observed player

- **WHEN** 载荷同时含 `player`（含弹药）与 `allplayers`
- **THEN** 被观察者出现一次且保留弹药信息，其它选手来自 `allplayers`

#### Scenario: Dead player

- **WHEN** 选手 `health = 0`
- **THEN** 该选手 `alive` 为 false

### Requirement: GSI HTTP endpoint

系统 SHALL 通过本地 HTTP 端点接收 GSI 推送：

- 非 POST 请求 SHALL 返回 405。
- 配置了 token 时，`Authorization: Bearer <token>` 不匹配 SHALL 返回 401 且不更新状态。
- 载荷超过 1MB SHALL 被截断读取；无法解析 SHALL 返回 400。
- 成功 SHALL 返回 200，并让订阅者与回调收到新状态。

#### Scenario: Unauthorised push

- **WHEN** 未带正确 token 推送
- **THEN** 返回 401 且最新状态不变

#### Scenario: Broken payload

- **WHEN** 推送 `{not json`
- **THEN** 返回 400

### Requirement: GSI integration config

系统 SHALL 生成 CS2 可读取的 GSI 配置文件：

- 格式 SHALL 为 Valve KeyValues（`"key" "value"`），SHALL NOT 为 JSON。
- SHALL 指向 `http://<listen>/`，并开启 provider/map/round/player/allplayers 相关 data 项。
- 指定 token 时 SHALL 输出 `auth.token`；未指定 SHALL 省略该块。

#### Scenario: Token block

- **WHEN** 以 token "abc" 生成配置
- **THEN** 配置含 `"token" "abc"` 且花括号平衡

### Requirement: Live state surfacing

`strikeye-cli gsi` SHALL 把每条载荷转成一行摘要并镜像最新状态：

- 摘要 SHALL 含 `map`、`phase`、`round`、`score`、`round_phase`、`bomb`、`players`、`alive_ct`、`alive_t`、`round_time_left`。
- 最新状态 SHALL 原子写入 `cache/gsi_latest.json`（`.part` → rename）。
- `--once` SHALL 在收到第一条载荷后退出；超时未收到 SHALL 返回错误。

#### Scenario: Mirror file

- **WHEN** 推送一份 de_mirage 载荷
- **THEN** `cache/gsi_latest.json` 含该地图与被观察选手

#### Scenario: Once without payload

- **WHEN** 以 `--once --timeout 150ms` 启动且无人推送
- **THEN** 返回 "no gsi payload" 错误
