# Tasks

- [x] `.github/workflows/ci.yml`：格式 / vet / test / 三平台构建 / spec 校验
- [x] openspec CLI 版本钉在 1.15.0（`OPENSPEC_VERSION`）
- [x] 语料回归改为 `workflow_dispatch` 可选任务
- [x] 仓库内 8 个未格式化文件 `gofmt -w`
- [x] 本地复跑 CI 全部命令：`gofmt -l` 为空、`go vet`、`go test -count=1`、darwin/windows 交叉编译、`openspec validate --all --strict`
- [x] README「Build & Release」补充 CI 说明
- [x] 报告 §4.10 与 §8 更新
