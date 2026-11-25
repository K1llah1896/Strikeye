# 分享码导入

## 为什么

分享码是玩家之间交换比赛的实际载体（CS:DM 也以它为主入口）。项目早已有 `sharecode` 解析与 `download` 下载，但两者在 GUI 里都不可达：用户拿到一串 `CSGO-…` 无处可贴。

## 变什么

1. Steam 下载面板新增「分享码」输入行：
   - 解析分享码得到 `match_id`；
   - 依次扫描五个 match history 标签，找到该场后下载到 `media/imports`；
   - 已存在则报"该场已导入"。
2. 分享码需要 Cookie 才能定位（match history 是登录态页面），缺少时直说而不是静默失败。
3. 状态栏回报比赛 ID、命中的标签页与文件名。

## 影响

- capability：`demo-download`（ADDED「Share code import」）。
- 复用 `internal/sharecode` 与 `internal/gcpd`，无新依赖。
