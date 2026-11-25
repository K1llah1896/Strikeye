# Spec Delta: native-ui-shell-and-guard

## ADDED Requirements

### Requirement: Settings installs tools from the package

设置页的工具安装入口 SHALL 默认从随包的边车安装，而不是从网络下载：

- 缺失 HLAE/FFmpeg 时按钮文案 SHALL 为“从内置包安装”；
- 点击后 SHALL 只在本地解压（无网络请求），并在完成后写入配置路径、重新探测、通知 agent 重载配置；
- 边车缺失时 SHALL 显示带具体文件名的提示与源码构建命令（`strikeye-cli assets-pack --tools-out <目录>`），而不是静默失败；
- “地图数据”卡片的一键安装 SHALL 同时安装地图数据与工具，并汇总两者结果。

#### Scenario: Offline install from the package

- **WHEN** 用户在设置页点击「从内置包安装 FFmpeg」，且边车存在
- **THEN** 无网络请求，`paths.ffmpeg_exe` 被写入内置路径，状态显示安装完成

#### Scenario: Missing sidecar explains itself

- **WHEN** 边车不存在时点击该按钮
- **THEN** 状态区显示“本安装包未附带 ffmpeg（期望文件名 ffmpeg-windows-amd64.zip）”及 `assets-pack --tools-out` 提示

#### Scenario: One-click map data also stages tools

- **WHEN** 用户在「地图数据」卡片点击一键安装
- **THEN** 摘要同时报告导航网格/callout/几何数量与工具安装结果
