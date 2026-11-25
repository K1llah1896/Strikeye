# Tasks

## 1. Bundled assets package

- [x] 1.1 新增 `internal/mapassets`：`go:embed all:assets`、`Available()`、`Install(dataRoot, exeDir)`
- [x] 1.2 内置 navs.zip 与 callouts.csv 安装：钉定 `bytes`+`sha256` 校验、`.part`→rename、按大小复用
- [x] 1.3 边车几何：`FindGeometryBundle`、`EnsureGeometry`、`ExtractGeometry`、`ExtractAllGeometry`、`BundleMaps`
- [x] 1.4 单元测试：安装/幂等/截断覆盖/篡改拒绝/缺边车降级/边车解压/校验失败不留残档/真实内置 nav 可解压
- [x] 1.5 `assets/README.md` 记录来源、体积与生成命令；`.gitignore` 排除 blob

## 2. Build input pipeline

- [x] 2.1 `strikeye-cli assets-pack`：下载 navs/callouts/几何并校验，输出 blobs 与 `map_geometry.zip`
- [x] 2.2 生成 blob 并实测：navs 7,683,060B、callouts 780,350B、几何包 199,678,716B（16 图）
- [x] 2.3 发布流水线：编译前执行 `assets-pack`，发布 zip 含 `map_geometry.zip`

## 3. Runtime wiring

- [x] 3.1 CLI `resolveDataRootOrExit` 后自动安装内置数据
- [x] 3.2 GUI `NewApp` 后台安装并通知；设置页卡片展示内置状态 + “从内置包安装”
- [x] 3.3 分析页（区域控制、球队档案）读取前确保安装，缺失提示改源码构建指引
- [x] 3.4 几何按需：`mapgeom.LoadForMap`/`LoadForMapWithBundle`、`parser.ParseOptions.GeometryBundle`
- [x] 3.5 热更新落盘 `map_geometry.zip`（0644，缺文件保持原样）+ 单测

## 4. Verification

- [x] 4.1 `go build ./...`、`go vet ./...`、`gofmt -l` 干净、`go test ./...` 全绿
- [x] 4.2 空 DataRoot 实测：`navs` 安装 22 张、`area`/`navpath` 离线可用（101 跳 / 5168.58u 与联网结果一致）
- [x] 4.3 空 DataRoot 解析 dust2：只解压 `de_dust2.tri`，TTD 与联网几何一致（KSCERATO 483.2ms/27、NiKo 940.9ms/23）
- [x] 4.4 `STRIKEYE_CORPUS=1 go test ./internal/corpus/` 用边车几何复现金标快照 0 差异
- [x] 4.5 `openspec validate bundled-map-assets --strict`
