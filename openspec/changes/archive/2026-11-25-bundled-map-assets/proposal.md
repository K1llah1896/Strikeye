# Change: Bundle Every Map Dataset Inside the Package

## Why

导航网格、callout 表与碰撞几何现在都要用户先点“下载”或敲 `navs-fetch` / `maps-fetch` / `callouts-fetch`，否则区域控制、地图控制曲线、TTD、击杀视线这些能力在全新安装上是空的。用户明确要求：需要下载的资源全部整合进安装包，不要让人去点下载。

三份数据的体积差别很大，因此用两种机制装进包内：

- `navs.zip`（7.7 MB）与 `nav_places.csv`（780 KB）足够小，用 `go:embed` 直接编进可执行文件，首次运行时写入 `<DataRoot>/cache/`。
- 碰撞几何 `.tri` 合计 200 MB，编进 exe 会让每个二进制膨胀数倍，改为随包提供 `map_geometry.zip` 边车文件（放在 exe 同目录），按需按图解压——只有真正用到的地图才会落盘。

两种路径都保持既有的校验和与原子写入约定，并保留原下载命令作为“升级到更新数据集”的手段，而不是前提条件。

## What Changes

- 新增 `internal/mapassets`：把 navs.zip 与 callouts.csv 编入二进制；`Install` 校验钉定 sha256 后写到 `<DataRoot>/cache/map_nav`、`<DataRoot>/cache/map_data/callouts.csv`；`EnsureGeometry`/`ExtractGeometry`/`ExtractAllGeometry` 从 `map_geometry.zip` 按图解压并逐条校验 manifest。
- `go:embed` 的资源目录入库只有 README 占位，二进制 blob 由新增的 `strikeye-cli assets-pack` 在打包前生成（`--asset-dir`、`--geometry-out`、`--skip-nav`、`--skip-geom`），并按钉定校验和下载。
- `mapgeom.LoadForMap` / `LoadForMapWithBundle`：几何首次使用时自动从边车解压；`parser.ParseOptions.GeometryBundle` 让回归语料也能按需取图。
- CLI 在解析 DataRoot 时（`resolveDataRootOrExit`）自动安装内置数据；GUI 启动后台安装并给出通知；设置页「地图数据」卡片改为展示内置状态并保留“从内置包安装／联网更新”按钮。
- 分析页（区域控制、球队档案）在加载前先确保内置数据就位，缺失提示改为指引“源码构建请先 `assets-pack`”。
- 更新器（`internal/updater`）在热更新时一并落盘 `map_geometry.zip`，保证升级后仍无需下载。
- 发布流水线在编译前执行 `assets-pack`（嵌入需要文件已存在），并把 `map_geometry.zip` 打进发布 zip。

## Impact

- Affected specs: `bundled-map-data`（新增）、`map-geometry-cache`（修改）、`match-library-and-analysis-ui`（修改）
- Affected code: `internal/mapassets`（新增）、`internal/mapgeom`、`internal/parser`、`internal/corpus`、`internal/updater`、`internal/ui`、`cmd/strikeye-cli`、`.github/workflows/release.yml`、`.gitignore`
- 体积：exe 各增加约 8.5 MB（navs + callouts）；发布 zip 增加约 200 MB（碰撞几何）。
- 许可：nav 与几何数据由 Valve 地图资产衍生，callout 表为 MIT；二进制仍不入库（`.gitignore`），只在构建时从钉定来源取得并校验。
