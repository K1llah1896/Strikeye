# gsi-live

## ADDED Requirements

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
