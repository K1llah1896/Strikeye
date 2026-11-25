# Tasks

## 1. Platform report

- [x] 1.1 `platformReport` + `describePlatform()`
- [x] 1.2 构建标签常量 `ipcSocketKind()`
- [x] 1.3 `doctor` 输出 `platform` 段

## 2. Verification

- [x] 2.1 `TestDescribePlatform`（OS/arch/socket/HLAE 标志一致）
- [x] 2.2 `GOOS=linux|darwin|windows go build ./...` 全部通过
- [x] 2.3 真实 `doctor` 输出含平台说明
- [x] 2.4 `go test ./...`、`openspec validate platform-parity --strict`
