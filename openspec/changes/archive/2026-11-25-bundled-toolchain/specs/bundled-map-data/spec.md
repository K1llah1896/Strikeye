# Spec Delta: bundled-map-data

## ADDED Requirements

### Requirement: Bundled external tool archives

FFmpeg 与 HLAE SHALL 以边车压缩包随安装包分发，并在启动时自动解压到 DataRoot，使用户无需点击任何下载按钮即可渲染、合成与录制：

- 边车文件名 SHALL 为 `ffmpeg-windows-amd64.zip` 与 `hlae-windows-amd64.zip`；
- 查找顺序 SHALL 为：可执行文件目录、其 `assets/` 子目录、工作目录、DataRoot 及其 `cache/`、`assets/` 子目录；
- 解压目标 SHALL 为 `<DataRoot>/tools/<tool>/`，二进制探测复用既有安装器的多层探测；
- 工具已就绪时 SHALL NOT 重新解压，且边车缺失时 SHALL NOT 影响启动；
- 边车不存在时 SHALL 报告“缺失”而不是报错；
- 热更新 SHALL 把两个边车作为数据文件落盘（0644，不带执行权限）。

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
