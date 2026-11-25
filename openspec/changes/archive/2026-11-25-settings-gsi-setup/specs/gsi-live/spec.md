# gsi-live

## ADDED Requirements

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
