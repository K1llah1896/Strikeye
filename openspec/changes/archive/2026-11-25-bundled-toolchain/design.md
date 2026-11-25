# Design: bundled-toolchain

## 分发形态

| 资源 | 体积 | 载体 | 原因 |
| --- | --- | --- | --- |
| 导航网格 + callout | 8.5 MB | `go:embed` 进 exe | 小，且每个功能都用到 |
| 碰撞几何 | 200 MB | `map_geometry.zip` 边车 | 太大，且按图按需解压 |
| FFmpeg | 119 MB | `ffmpeg-windows-amd64.zip` 边车 | 单文件上百 MB，三 exe 各带一份不可接受 |
| HLAE | 9 MB | `hlae-windows-amd64.zip` 边车 | 仅 Windows 录制需要 |

边车统一放在可执行文件目录，热更新通过 `updater` 的 data-file 白名单落盘（0644，不带执行权限）。

## 为什么用钉定直链而不是 Release API

`resolveGitHubZipAsset` 走 `api.github.com/repos/.../releases/latest`，未认证配额 60 次/小时。
打包机器上实测直接失败：

```
获取 HLAE 下载地址失败: HTTP 403
{"message":"API rate limit exceeded for 103.156.242.227. ..."}
```

因此打包路径改为钉定直链，构建可复现且不吃配额：

- HLAE `https://github.com/AdvancedFX/advancedfx/releases/download/v2.193.2/hlae_2_193_2.zip`
- FFmpeg `https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-win64-gpl.zip`

应用内仍保留联网安装作为兜底（`updater.DownloadToolArchive`，`releaseAPI` 参数可覆盖）。

## 解压与探测

- 解压直接复用 `updater.InstallToolFromArchive`：ZIP 内的目录层级任意（FFmpeg 是
  `ffmpeg-9.1-essentials_build/bin/ffmpeg.exe`，HLAE 是 `hlae/HLAE.exe`），由
  `DetectToolBinaries` 逐层探测出 `ffmpeg`/`ffprobe`/`HLAE.exe`/`AfxHookSource2.dll`。
- `toolbundle.Provision` 幂等：`toolStaged` 判据是“目录存在且 `DetectToolBinaries` 成功”，
  已就绪的工具既不重解压也不要求边车存在（用户删掉边车后仍能启动）。
- 边车缺失只记 `Missing`，不返回错误：源码构建依旧可以跑，设置页保留联网兜底。

## 探测发现（config）

`ProbeToolchain` 原本只看 `Configured`、`ExtraSearchDirs`、`PATH` 与平台默认位置，
解压后的二进制定位不到。因此新增 `ProbeOptions.DataRoot` 并加 `stagedToolCandidates`：

```
<DataRoot>/tools/<tool>/<exe>
<DataRoot>/tools/<tool>/bin/<exe>
<DataRoot>/tools/<tool>/*/<exe>
<DataRoot>/tools/<tool>/*/bin/<exe>
<DataRoot>/tools/<tool>/*/*/bin/<exe>
```

用 glob 而非 walk，避免每次探测都遍历 FFmpeg 目录里的上千个文件。候选顺序放在
`PATH` 之前，保证“包内自带的那份”优先于系统安装的版本。

## 验收路径

1. `assets-pack --skip-nav --skip-geom --tools-out assets` 产出两个边车。
2. 空 DataRoot + 边车 → 任意 CLI 命令即自动安装，`doctor` 报告
   `hlae=.../tools/hlae/HLAE.exe`、`afx_hook_source2=.../tools/hlae/x64/AfxHookSource2.dll`、
   `ffmpeg=.../tools/ffmpeg/ffmpeg-9.1-essentials_build/bin/ffmpeg.exe`。
3. 删除边车后重跑：`installed bundled tools` 不再出现，但探测结果不变（幂等）。
