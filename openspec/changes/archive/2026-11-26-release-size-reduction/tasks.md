# Tasks

## 1. Geometry bundle
- [x] 1.1 `mapgeom.CoreMaps` + `IsCoreMap`，`assets-pack` 默认只打竞技图
- [x] 1.2 `assets-pack --all-geometry` 保留全量打包
- [x] 1.3 条目改为 xz(原始 `CSDAGEO` 负载)，manifest 增 `encoding`
- [x] 1.4 `mapgeom.decompressGeometry` 支持 gzip/zstd/xz/plain 四种容器
- [x] 1.5 设置页“补充其他地图几何（休闲图）”按钮
- [x] 1.6 测试：xz/zstd 条目解压与校验、四种容器读取、缺图/坏包

## 2. FFmpeg sidecar
- [x] 2.1 `toolbundle.PackFFmpeg`：只保留 ffmpeg.exe/ffprobe.exe，单个 zstd 固态帧
- [x] 2.2 `updater.ExtractToolArchive` 支持 `.tar.zst` 与旧 `.zip`
- [x] 2.3 边车名改 `ffmpeg-windows-amd64.tar.zst`，旧名继续被识别
- [x] 2.4 钉定上游为 gyan.dev essentials（110 MB 二进制，而非 master 的 172 MB）
- [x] 2.5 测试：打包只留两个二进制、缺 ffprobe 报错、tar.zst 安装、旧名优先顺序

## 3. Binaries and packaging
- [x] 3.1 发布构建加 `-s -w`
- [x] 3.2 打包分 `zip -9`（exe）与 `zip -0`（边车），并 `zip -T` 校验
- [x] 3.3 `.gitignore` 增新边车名，`updater` 数据文件白名单同步

## 4. Fonts
- [x] 4.1 用 `scripts/subset-fonts.sh` 生成 GBK 子集（每字重 7.7 MB）
- [x] 4.2 `assets/fonts/coverage_test.go`：覆盖源码内 CJK/标点码位，缺字即失败

## 5. Verification
- [x] 5.1 `go test ./... -count=1` 全绿
- [x] 5.2 空 DataRoot + 边车实测：内置 nav/callout/工具安装、几何按图解压
- [x] 5.3 真实 demo 重解析：`killctx` 平均交火 760.9 单位、遮挡 20.4%、穿烟 7.3%（与压缩前一致）
- [x] 5.4 打包实测：`strikeye-windows-amd64.zip` 527 MB → 255 MB
- [x] 5.5 文档：README 与调研报告记录新的体积构成与来源
