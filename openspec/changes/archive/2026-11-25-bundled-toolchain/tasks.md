# Tasks: bundled-toolchain

## 1. 工具边车安装器

- [x] 1.1 `internal/updater/tools.go` 抽出 `DetectToolBinaries` / `InstallToolFromArchive` / `DownloadArchiveURL`，`DownloadToolArchive` 支持 `releaseAPI` 覆盖
- [x] 1.2 新增 `internal/toolbundle`：钉定文件名与直链、`Dirs`/`Find`/`StagedPath`、`Provision`（幂等、缺失不报错）、`Report.Summary`
- [x] 1.3 `internal/toolbundle/toolbundle_test.go` 覆盖解压、幂等、缺失、坏包、DataRoot 搜索

## 2. 打包与发布

- [x] 2.1 `assets-pack` 增加 `--tools-out` / `--skip-tools` / `--tools-api-url` 与 `packTools`
- [x] 2.2 发布流水线生成两个边车并打进 zip；`updater` 数据文件白名单加入两个边车
- [x] 2.3 `assets-pack` 复用已存在边车（`assets_pack_test.go`）

## 3. 自动安装接线

- [x] 3.1 CLI `resolveDataRootOrExit` 在安装地图数据后调用 `toolbundle.Provision`
- [x] 3.2 GUI `NewApp` 后台安装工具并发出通知
- [x] 3.3 设置页按钮改为「从内置包安装」，抽出 `applyToolResult`，地图卡片汇总工具结果

## 4. 探测发现

- [x] 4.1 `ProbeOptions.DataRoot` + `stagedToolCandidates` glob 候选
- [x] 4.2 全部 `ProbeToolchain` 调用点传入 DataRoot

## 5. 验证

- [x] 5.1 `go build ./...`、`go vet ./...`、`go test ./...` 全绿
- [x] 5.2 真实边车：`assets-pack --skip-nav --skip-geom --tools-out assets` 产出 119 MB + 9 MB
- [x] 5.3 空 DataRoot + 边车：`installed bundled tools: ffmpeg, hlae`，`doctor` 报告三个工具路径
- [x] 5.4 `openspec validate bundled-toolchain --strict` 通过
