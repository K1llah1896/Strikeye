# 持续集成门禁

## 为什么

仓库只有 tag 触发的 release 工作流：代码在推到分支、开 PR 时不跑任何检查。解析器、指标与 schema 迁移都已经进入"改一个常量就改一版指标"的阶段，靠人工记得跑 `go test` 和 `openspec validate` 并不可靠——一次漏跑就会把指标口径变更混进发布。

## 变什么

1. 新增 `.github/workflows/ci.yml`，在 push 到任意分支与 PR 时执行：
   - `gofmt -l ./cmd ./internal` 必须为空；
   - `go vet ./...`；
   - `go test ./... -count=1`；
   - 三平台交叉编译 `linux/amd64`、`darwin/arm64`、`windows/amd64`；
   - `openspec validate --all --strict`（CLI 版本钉在 1.15.0，与 spec 编写时一致）。
2. 语料回归（`strikeye-cli corpus`）作为 `workflow_dispatch` 可选任务：它要下载地图几何，跑在 PR 上会让门禁依赖外网。
3. 顺手把仓库里 8 个未格式化的文件 `gofmt` 掉，否则格式门禁一上线就是红的。

## 影响

- capability：新增 `ci-pipeline`。
- 门禁依赖 `@fission-ai/openspec@1.15.0` 与 Node 22；版本升级需与 spec 一起验证。
