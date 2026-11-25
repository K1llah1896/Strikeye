# Design: 选手头像抓取与展示

## Context

SteamID64 已经在 `match_players` 里；布局模块 `internal/config` 已定义 `CacheRadarPortraits`。缺的是一条"从 SteamID 到本地图片文件"的管线，以及一个渲染位。

## Decisions

1. **用公开 XML 资料页而不是 Web API**：`ISteamUser/GetPlayerSummaries` 需要 API key（`vac-risk` 已存在 key 的可选依赖），而 XML 资料页匿名可读，不需要让用户为了头像再配一次 key。代价是要解析 CDATA，且资料页比 API 慢——用 20 秒超时和 512KB 上限兜住。
2. **不新增依赖**：解析用正则而非 XML 解码器，因为资料页里 `<avatarFull>` 一定是 CDATA 单行，正则足够稳；下载只用 `net/http`。
3. **30 天 TTL**：Steam 头像更换不频繁，30 天既避免每次启动都打网络，又不会把旧头像留太久；`--refresh` 供强制更新。
4. **`.part` → rename**：与仓库其它下载管线（几何、导航网格、GSI 镜像）保持一致，进程中断不会留下半个文件。
5. **UI 侧只读缓存、不做网络请求**：渲染路径不能阻塞在 HTTP 上；没有缓存就回退到阵营色圆点，功能降级不可见。
6. **失败不致命**：批量抓取把每个 SteamID 的失败记进 `failed[]`，退出码仍为 0；单个私密资料不该让整批失败。

## Risks

- Steam 偶发 429/403：客户端返回错误文本，CLI 汇总在 `failed[]`，重跑即可。
- 头像格式为 JPEG 时扩展名保留 `.jpg`，UI 侧按 `.png|.jpg|.jpeg` 三种扩展查找。
