# player-profiles delta

## ADDED Requirements

### Requirement: Roster portraits fetch automatically

分析页打开一场比赛时 MUST 自动补齐缺失的选手头像，用户无需点击任何按钮。

#### Scenario: Missing portraits are fetched on demo open

- **WHEN** 用户打开一场比赛，且部分选手在 `<DataRoot>/cache/radar_cards/portraits` 下没有头像文件
- **THEN** 视图在后台按 SteamID 抓取缺失头像（30 天 TTL），写盘后重新解码位图并重绘选手表
- **AND** 已有缓存的选手不产生任何网络请求

#### Scenario: Automatic fetch can be disabled or faked

- **WHEN** `View.avatarAutoFetch` 为 false，或注入了 `View.avatarClient`
- **THEN** 自动抓取不启动，或改用注入的客户端
- **AND** 渲染测试因此在无网络环境下保持确定性
