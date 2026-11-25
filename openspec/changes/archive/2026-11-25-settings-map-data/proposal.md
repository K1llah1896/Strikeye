# 设置页地图数据入口

## 为什么

区域控制、地图控制曲线、击杀区位与 TTD 都依赖 `.nav` 与 callout CSV。缺数据时这些页面会提示"运行 `strikeye-cli navs-fetch`"——让桌面应用的用户去开终端，是能力与界面之间的断点。

## 变什么

1. 设置页新增「地图数据（导航网格与 callout）」卡片：
   - 显示已缓存导航网格张数、callout 文件大小、最近检查时间；
   - 「下载 callout」与「下载导航网格（全部官方图）」两个按钮，后台下载并通过顶部状态条回报；
   - 「刷新状态」按需重新采样缓存。
2. 下载沿用 `internal/mapdata`（校验和、`.part`→rename、按需解压），不复制任何下载逻辑。
3. 下载任务并发保护：进行中显示「下载中...」，按钮不会重复触发。

## 影响

- capability：`match-library-and-analysis-ui`（ADDED「Map data cache controls」）。
- 无新表、无新配置项：缓存目录固定为 `<DataRoot>/cache/map_nav` 与 `<DataRoot>/cache/map_data/callouts.csv`。
