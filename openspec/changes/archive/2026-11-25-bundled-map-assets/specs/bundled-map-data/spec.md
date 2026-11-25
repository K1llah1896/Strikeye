# Spec Delta

## Purpose

定义随安装包一起分发的离线地图数据集：导航网格、callout 地名表与碰撞几何，使全新安装在无网络、无点击的前提下即可使用区域、控制曲线与视线相关能力。

## ADDED Requirements

### Requirement: Embedded navigation and callout datasets

系统 SHALL 把导航网格与 callout 表编入可执行文件，并在首次运行时写入 DataRoot 缓存：

- `navs.zip`（awpy-data release `2000950`）与 `nav_places.csv`（`cs2-server-admin-callouts` 钉定 commit）在解压前 SHALL 按钉定 `bytes`+`sha256` 校验；
- 校验失败 SHALL 返回错误且不写入任何缓存文件；
- 已存在且大小正确的目标文件 SHALL 复用，不重复写入；
- 目标路径 SHALL 为 `<DataRoot>/cache/map_nav/<map>.nav` 与 `<DataRoot>/cache/map_data/callouts.csv`。

#### Scenario: Fresh install gets nav meshes without network

- **WHEN** 在空的 DataRoot 上运行任意 CLI 子命令或启动 GUI
- **THEN** `<DataRoot>/cache/map_nav/` 出现内置的 `.nav` 文件，`navpath`/`area`/`zones` 无需联网即可工作

#### Scenario: Tampered blob is rejected

- **WHEN** 内置 blob 与钉定 sha256 不符
- **THEN** 安装返回 `ErrChecksum`，`cache/map_nav` 与 `callouts.csv` 都不会被写入

#### Scenario: Repeat install is a no-op

- **WHEN** 内置数据已经安装完毕再次启动
- **THEN** 不重写任何文件，报告显示 `navs_written=0` 且 callouts 未重写

### Requirement: Sidecar collision geometry bundle

碰撞几何 SHALL 作为 `map_geometry.zip` 边车随包分发，并按图按需解压：

- 边车 SHALL 位于可执行文件同目录（或 DataRoot 及其 `cache`/`assets` 子目录）；
- 每个 `.tri` 在解压时 SHALL 按边车 `manifest.json` 的 `size`+`sha256` 校验；
- 已存在的文件大小正确时 SHALL 复用，不重复解压；
- 边车缺失时 SHALL 退化为“几何不可用”，不影响解析与其他指标。

#### Scenario: Parsing extracts only the needed map

- **WHEN** 在一个没有几何缓存的 DataRoot 上解析 `de_dust2` demo，且 exe 旁存在 `map_geometry.zip`
- **THEN** 只有 `de_dust2.tri` 被解压进缓存，TTD 与击杀视线指标正常产出

#### Scenario: Corrupt bundle entry leaves nothing behind

- **WHEN** 边车中某张图的校验和与 manifest 不符
- **THEN** 该次调用报告无几何可用，缓存目录不留下 `.part` 或半成品 `.tri`

#### Scenario: Missing bundle degrades gracefully

- **WHEN** 既无缓存也无边车
- **THEN** 解析成功、`geometry_used=false`，依赖几何的指标为 0 而不是报错

### Requirement: Asset bundle build input

仓库 SHALL NOT 提交地图数据二进制；`strikeye-cli assets-pack` SHALL 是生成内置与边车数据集的唯一入口：

- 按钉定来源下载并校验后写出 `internal/mapassets/assets/{navs.zip,callouts.csv}` 与 `<geometry-out>/map_geometry.zip`；
- 支持 `--skip-nav`、`--skip-geom`、`--asset-dir`、`--geometry-out`、`--geometry-base-url`；
- 发布流水线 SHALL 在编译前执行该命令，源码构建缺少 blob 时 SHALL 仍然编译通过并退化为“未内置”。

#### Scenario: Release build embeds the datasets

- **WHEN** 发布流水线打 tag 构建
- **THEN** 编译前 `assets-pack` 生成 `navs.zip`、`callouts.csv` 与 `map_geometry.zip`，产物 zip 同时包含三个 exe 与几何边车

#### Scenario: Source build without blobs compiles

- **WHEN** 在没有 blob 的 checkout 上执行 `go build ./...`
- **THEN** 编译成功，`mapassets.Available()` 返回 false，界面与 CLI 提示“本构建未内置地图数据”

### Requirement: In-app update keeps bundled assets in sync

热更新 SHALL 同时落盘 `map_geometry.zip`：

- 更新包内的 `map_geometry.zip` SHALL 被写入目标目录；
- 该文件 SHALL NOT 带执行权限；
- 更新包不含该文件时 SHALL 保留现有文件不变。

#### Scenario: Update installs geometry sidecar

- **WHEN** 应用一个包含 `map_geometry.zip` 的更新包
- **THEN** 目标目录中该文件被替换为新内容，权限为非可执行，其他可执行文件照旧热替换
