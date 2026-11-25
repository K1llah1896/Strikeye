# gsi-live Specification

## Purpose
用 Valve 官方的只读推送（GSI）把"正在进行的比赛"接进工具：本地接收端解析比分、回合、炸弹与存活人数，生成游戏侧配置，并把最新状态镜像成文件供 UI/脚本消费，无需注入或插件。

## Requirements

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

### Requirement: Configuration card in settings

设置页 SHALL 提供 GSI 配置生成与状态展示：

- 「生成 GSI 配置」SHALL 用 `gsi.Config` 渲染 KeyValues 并写入 `<DataRoot>/cache/gsi/gamestate_integration_strikeye.cfg`。
- 监听地址 SHALL 接受 `host:port`，缺少端口时 SHALL 报错且不写文件。
- 状态 SHALL 依据 `<DataRoot>/cache/gsi_latest.json` 的 `received_at` 判断，超过 30 秒 SHALL 视为无活动流。
- 有活动流时 SHALL 显示地图、回合、比分、回合阶段与存活人数。
- 卡片 SHALL 说明配置文件需要复制到游戏的 `cfg` 目录，且接收端需要常驻进程。

#### Scenario: Generate config

- **WHEN** 用户填写 `127.0.0.1:3000` 与 token 并点击「生成 GSI 配置」
- **THEN** cfg 文件出现在 `cache/gsi/`，内容包含该 URI 与 token

#### Scenario: Live state shown

- **WHEN** `gsi_latest.json` 的 `received_at` 在 30 秒内
- **THEN** 卡片显示 live 徽章与地图/回合/比分/存活人数

#### Scenario: Stale state

- **WHEN** 镜像文件是一小时前的
- **THEN** 卡片显示「无活动流」而不是旧数据

### Requirement: In-app receiver

桌面应用 SHALL 自带 GSI 接收端：

- 设置页 SHALL 提供启动/停止按钮，并显示实际监听地址。
- 接收端 SHALL 校验 `Authorization: Bearer <token>`（配置了令牌时），并拒绝非 POST。
- 每次状态更新 SHALL 原子写入 `<DataRoot>/cache/gsi_latest.json`（`.part` → rename），供资料库横幅读取。
- 应用关闭 SHALL 停止监听并释放端口。
- 同一视图重复启动 SHALL 返回错误而不是重复绑定。

#### Scenario: Start from settings

- **WHEN** 用户在设置页点击「启动内置接收端」
- **THEN** 卡片显示监听地址，向该地址 POST 一份 state 后资料库横幅出现地图/比分/存活信息

#### Scenario: Stop releases port

- **WHEN** 用户点击「停止接收端」或关闭应用
- **THEN** 端口释放，再次启动不报 "address already in use"

#### Scenario: Token enforcement

- **WHEN** 配置了令牌但请求未带 `Authorization`
- **THEN** 返回 401，状态不入库
