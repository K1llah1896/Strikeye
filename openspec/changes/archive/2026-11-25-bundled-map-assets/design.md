# Design: Bundled Map Assets

## 目标

全新安装（解压 zip 直接运行）在离线状态下即可完成：解析 demo、TTD 与击杀视线、区域控制、地图控制曲线、球队档案/倾向报告。用户不应被迫点击下载或敲 fetch 命令。

## 体积决策

| 数据 | 压缩后体积 | 机制 | 落盘位置 |
|---|---|---|---|
| `navs.zip`（22 张图） | 7.7 MB | `go:embed` 编入二进制 | `<DataRoot>/cache/map_nav/<map>.nav` |
| `nav_places.csv` | 780 KB | `go:embed` 编入二进制 | `<DataRoot>/cache/map_data/callouts.csv` |
| `de_*.tri`（16 张图） | 199.7 MB | 边车 `map_geometry.zip` | `<DataRoot>/cache/map_geometry/<map>.tri` |

把 200 MB 编进每个 exe 会让 GUI/agent/CLI 三个二进制各膨胀三倍以上，因此几何走边车。边车仍是“包内”的一部分：它在发布 zip 里，与 exe 同目录，用户无需做任何操作。三个 exe 各 +8.5 MB 是可接受的代价（当前 GUI 约 75 MB）。

## 安装时机

- CLI：`resolveDataRootOrExit` 之后立即 `mapassets.Install`（幂等，已安装时只做几次 `stat`）。
- GUI：`NewApp` 里后台 goroutine 安装，成功/失败各发一条系统通知，不阻塞首帧。
- 分析页：区域控制与球队档案在读取 callouts/`.nav` 前再调一次 `Install`，覆盖 headless 或安装被跳过的情况。
- 几何：真正需要时才解压单图（`mapgeom.LoadForMap`），首次 `parse`（TTD）或打开击杀区位时触发；一张图 6–20 MB，解压 <200 ms。

## 校验与原子性

沿用既有约定：

- 内置 blob 在解压前按 `NavsBytes/NavsSHA256`、`CalloutsBytes/CalloutsSHA256` 校验；不匹配返回 `ErrChecksum` 且不写任何文件（`TestInstallRejectsTamperedBlob` 覆盖）。
- 每个文件写 `<dest>.part` 再 `rename`；`.nav`/`.csv` 已存在且大小正确时跳过；`.tri` 逐条校验边车 `manifest.json` 里的 `size`+`sha256`。
- 边车 manifest 与上游 `cs2-map-geometry` 的 `manifest.json` 同构，因此下载命令与内置路径共用一份校验口径。

## 构建输入而非源码

`.gitignore` 排除 `internal/mapassets/assets/*.zip|*.csv` 与 `assets/map_geometry.zip`；`assets/README.md` 记录来源与生成命令。`strikeye-cli assets-pack` 是唯一的生成入口，发布流水线与本地打包都调用它，保证钉定版本只有一个真源（`mapdata.NavRelease`、`NavBaseURL`、`CalloutsURL`、`mapgeom.DefaultBaseURL`）。

源码构建若没有 blob，`mapassets.Available()` 返回 false，所有调用点退化为“数据缺失”，不会编译失败——`go:embed all:assets` 至少匹配到 README。

## 热更新

`internal/updater.applyZipPackage` 原来只接受三个可执行文件名；现在额外接受 `map_geometry.zip` 并以 0644 落盘（数据文件不应带执行位）。旧版本发布包没有该文件时行为不变。

## 回归语料

`corpus.BuildOptions.GeometryBundle` 让语料快照也能用边车几何；`TestFixtureCorpus` 在 `dist/cache/map_geometry` 为空时自动使用 `assets/map_geometry.zip`，因此金标指标（含 TTD 样本数/均值）在全新 checkout 上依然可复现。
