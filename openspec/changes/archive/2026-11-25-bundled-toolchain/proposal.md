# bundled-toolchain

## Why

地图数据已经内置，但 FFmpeg 与 HLAE 仍要在设置页点「下载安装」才可用：它依赖 GitHub
Release API（未认证时每小时 60 次，实测会直接回 `HTTP 403 API rate limit exceeded`），
并且全新安装在无网络环境下无法渲染、合成或录制。用户要求“需要下载的资源全部整合到包内，
不要让用户去点击下载”，所以外部工具也必须随包分发。

## What Changes

- 新增 `internal/toolbundle`：定位随包分发的 `ffmpeg-windows-amd64.zip` /
  `hlae-windows-amd64.zip` 边车、幂等解压到 `<DataRoot>/tools/<tool>`，复用既有
  `updater.InstallToolFromArchive` 的二进制探测。
- `assets-pack` 增加 `--tools-out` / `--skip-tools` / `--tools-api-url`，用钉定直链
  （HLAE `v2.193.2`、FFmpeg BtbN `latest`）下载工具包，避开 GitHub API 限流。
- 启动时自动安装：CLI 的 `resolveDataRootOrExit` 与 GUI `NewApp` 在地图数据之后运行
  `toolbundle.Provision`，无需任何点击。
- `config.ProbeToolchain` 增加 `ProbeOptions.DataRoot`，在
  `<DataRoot>/tools/<tool>[/<build>][/bin]/<exe>` 中 glob 已解压的二进制，使
  FFmpeg/HLAE/AfxHookSource2 在设置页与渲染链路中被自动识别。
- 设置页两个按钮从「下载安装」改为「从内置包安装」（离线、无网络），边车缺失时给出
  `assets-pack --tools-out` 的源码构建提示。
- 发布流水线与热更新把两个工具边车一起打包、一起落盘。

## Impact

- 新增能力依赖：`bundled-map-data`（新增工具边车要求）、`config-and-paths`（探测发现）、
  `native-ui-shell-and-guard`（设置页离线安装）。
- 发布包体积增加约 129 MB（FFmpeg 119 MB + HLAE 9 MB），全部走边车，不进可执行文件。
- 不再依赖 GitHub API 配额：打包与安装路径都是直链或本地文件。
