# Spec Delta

## MODIFIED Requirements

### Requirement: Bundled external tool archives

FFmpeg 与 HLAE SHALL 以边车压缩包随安装包分发，并在启动时自动解压到 DataRoot，使用户无需点击任何下载按钮即可渲染、合成与录制：

- 边车文件名 SHALL 为 `ffmpeg-windows-amd64.tar.zst` 与 `hlae-windows-amd64.zip`；旧版 `ffmpeg-windows-amd64.zip` SHALL 仍被接受；
- FFmpeg 边车 SHALL 只包含 `ffmpeg.exe` 与 `ffprobe.exe`（可含许可文件），并以单个 zstd 固态帧保存；打包时 SHALL 校验产物可解压且能探测到两个二进制；
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

## ADDED Requirements

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
