## 1. 来源注册表

- [x] 1.1 新增 `internal/watcher/sources.go`：18 个 canonical 平台常量、中文标签注册表、有序路径规则表
- [x] 1.2 保持 `ClassifyPlatform` 兼容并补目录结尾匹配（`/pw/`、`/esl/`）
- [x] 1.3 提供 `NormalizePlatform` 归一旧键（perfectworld/pw、5eplay）与 `PlatformIDs`/`AllPlatforms`/`PlatformLabel`

## 2. 归档导入

- [x] 2.1 新增 `internal/watcher/archive.go`：zip/gzip/bzip2/zstd 识别与解压
- [x] 2.2 解压到 `ImportDir/<archive>-<hash>`，仅保留 `.dem`、扁平化、去重、`.part` 原子落盘
- [x] 2.3 施加条目数/单条/总量限制与 `ErrArchiveLimit`/`ErrArchiveEmpty`
- [x] 2.4 watcher 将稳定归档解压后逐个走校验、哈希去重、入库与回调
- [x] 2.5 遍历时跳过 `ImportDir` 子树，避免重复扫描解压产物

## 3. 边车元数据

- [x] 3.1 新增 `internal/watcher/info.go`：容错解析 `.info`，归一日期与平台
- [x] 3.2 入库时用边车补齐平台/日期/队伍/比分/比赛 ID
- [x] 3.3 `ApplyIngestMetadata` 仅填空字段，不覆盖 ready 解析结果与用户备注

## 4. 库筛选与接线

- [x] 4.1 平台筛选选项由注册表生成，旧值经 `NormalizePlatform` 仍可匹配
- [x] 4.2 agent 与库内手动扫描使用 `<DataRoot>/media/imports`
- [x] 4.3 新增单测覆盖分类、解压安全、边车解析、watcher 端到端与元数据合并

## 5. 验证

- [x] 5.1 `go build ./...` 与 `go test ./...` 全绿
- [x] 5.2 `openspec validate demo-source-expansion --strict` 通过
