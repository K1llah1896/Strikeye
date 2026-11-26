# bundled-map-data Specification

## Purpose
定义随安装包一起分发的离线数据与外部工具：导航网格、callout 地名表、碰撞几何，以及 FFmpeg/HLAE 工具包，使全新安装在无网络、无点击的前提下即可使用区域、控制曲线、视线相关能力与录制/合成链路。

## Requirements

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
- 边车默认 SHALL 只包含竞技图池（`mapgeom.CoreMaps`）；`assets-pack --all-geometry` SHALL 能生成包含全部官方地图的版本；
- 每个条目 SHALL 保存 `CSDAGEO` 原始负载经 xz 或 zstd 压缩的帧，并在 `manifest.json` 中用 `encoding` 标注容器；未标注 SHALL 视为发布方的原始负载；
- 解压时 SHALL 按 `manifest.json` 的 `size`+`sha256` 校验解压后的 `.tri` 字节；
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

#### Scenario: Compressed entries inflate to the verified payload

- **WHEN** 边车条目以 xz 或 zstd 编码
- **THEN** 解压得到的 `.tri` 字节与 manifest 的 `size`+`sha256` 一致，且 `mapgeom.Load` 直接在缓存文件上工作（无需边车再次参与）

#### Scenario: Casual maps stay reachable

- **WHEN** 边车只带竞技图，而用户要分析 `cs_italy`
- **THEN** 设置页提供“补充其他地图几何”，联网补齐后该图的 TTD 与视线指标可用

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

### Requirement: Bundled external tool archives

FFmpeg 与 HLAE SHALL 以边车压缩包随安装包分发，并在启动时自动解压到 DataRoot，使用户无需点击任何下载按钮即可渲染、合成与录制：

- 边车文件名 SHALL 为 `ffmpeg-windows-amd64.tar.zst` 与 `hlae-windows-amd64.zip`；
- 查找顺序 SHALL 为：可执行文件目录、其 `assets/` 子目录、工作目录、DataRoot 及其 `cache/`、`assets/` 子目录；
- 解压目标 SHALL 为 `<DataRoot>/tools/<tool>/`，二进制探测复用既有安装器的多层探测；
- 工具已就绪时 SHALL NOT 重新解压，且边车缺失时 SHALL NOT 影响启动；
- 边车不存在时 SHALL 报告“缺失”而不是报错；
- 热更新 SHALL 把工具边车作为数据文件落盘（0644，不带执行权限）。

#### Scenario: Fresh install stages both tools without a click

- **WHEN** 可执行文件旁存在两个工具边车，且 `<DataRoot>/tools` 为空
- **THEN** 首个 CLI 子命令或 GUI 启动后 `<DataRoot>/tools/ffmpeg/*/bin/ffmpeg.exe` 与 `<DataRoot>/tools/hlae/HLAE.exe` 就位

#### Scenario: Second run does not re-extract

- **WHEN** 工具已就绪且边车已被删除
- **THEN** 启动成功、无解压动作，工具仍被探测到

#### Scenario: Missing sidecar degrades to a message

- **WHEN** 源码构建没有工具边车
- **THEN** 启动与功能均不失败，报告把该工具标记为缺失，并提示 `assets-pack --tools-out <目录>`

#### Scenario: Update ships tool sidecars

- **WHEN** 应用一个包含工具边车的更新包
- **THEN** 两个边车被写入目标目录，权限为非可执行

#### Scenario: Legacy zip sidecar still installs

- **WHEN** 目标目录只有旧版 `ffmpeg-windows-amd64.zip`
- **THEN** 启动时仍按该 zip 解压 ffmpeg/ffprobe，且不要求用户重新下载

### Requirement: FFmpeg sidecar contents

FFmpeg 边车 SHALL 只携带 Strikeye 实际执行的二进制，并以单个固态帧保存：

- 内容 SHALL 仅含 `ffmpeg.exe`、`ffprobe.exe` 与许可文件，`ffplay.exe` 与上游文档 SHALL NOT 打包；
- 压缩容器 SHALL 为单个 zstd 固态帧（`.tar.zst`），使两个二进制的跨文件冗余可被压缩；
- 打包步骤 SHALL 解压产物并用与首次启动相同的探测逻辑确认两个二进制可执行。

#### Scenario: Pack drops unused binaries

- **WHEN** 用上游压缩包运行 `assets-pack --tools-out <目录>`
- **THEN** 产物中不含 `ffplay.exe` 与文档，`ffmpeg.exe`/`ffprobe.exe` 解出后能被探测到

#### Scenario: Pack refuses an incomplete upstream

- **WHEN** 上游压缩包缺少 `ffprobe.exe`
- **THEN** 打包失败并指出缺失的二进制，不产出半成品边车

### Requirement: Tool archives are build inputs

`strikeye-cli assets-pack` SHALL 是生成工具边车的唯一入口，并 SHALL 使用钉定直链下载以避免 GitHub API 配额：

- 支持 `--tools-out`、`--skip-tools`、`--tools-api-url`；
- 已存在且非空的边车 SHALL 复用，不重复下载；
- 发布流水线 SHALL 在编译前生成两个边车并把它们打进发布 zip。

#### Scenario: Packaging reuses fetched archives

- **WHEN** 目标目录已存在两个边车并再次执行 `assets-pack`
- **THEN** 不发起下载，输出仍列出两个文件名

#### Scenario: Pinned URL avoids the API quota

- **WHEN** 打包机在未认证状态下生成工具边车
- **THEN** 下载走 `releases/download/...` 直链，不调用 `api.github.com`

### Requirement: Release package size budget

发布包 SHALL 保持“所有核心能力随包提供”的前提下压缩体积，并 SHALL 由可复现的打包步骤保证：

- 三个可执行文件 SHALL 以 `-ldflags "-s -w"` 链接；
- 打包 SHALL 对可执行文件用 `zip -9`、对已压缩的边车用 `zip -0`，并 SHALL 在打包后校验 zip 完整性；
- 内嵌 CJK 字体 SHALL 为子集，且 SHALL 由 `scripts/subset-fonts.sh` 可从上游字体重建；
- 新增 UI 文案使用的中日韩与标点字符 SHALL 在字体子集覆盖测试中校验，缺字时该测试 SHALL 失败。

#### Scenario: Release zip stays within budget

- **WHEN** 发布流水线构建 `strikeye-windows-amd64.zip`
- **THEN** 压缩包包含三个 exe、`map_geometry.zip`、`ffmpeg-windows-amd64.tar.zst`、`hlae-windows-amd64.zip`，且总大小不超过 300 MB

#### Scenario: Subset fonts cover new UI text

- **WHEN** 新增的界面字符串包含子集里没有的汉字或标点
- **THEN** `go test ./assets/fonts/` 失败并指出缺失的码位
