# ci-pipeline

## ADDED Requirements

### Requirement: Pull request gate

每次 push 到任意分支与每个 pull request SHALL 触发同一套检查，且全部通过才算可合并：

- 格式检查 SHALL 对 `./cmd` 与 `./internal` 运行 `gofmt -l`，输出非空即失败。
- SHALL 运行 `go vet ./...`。
- SHALL 运行 `go test ./... -count=1`，超时上限不低于 20 分钟。
- SHALL 对 `linux/amd64`、`darwin/arm64`、`windows/amd64` 各执行一次 `go build ./...`。
- SHALL 运行 `openspec validate --all --strict`，且 openspec CLI 版本必须钉在 1.15.0。

#### Scenario: Unformatted file

- **WHEN** 提交里有一个未 `gofmt` 的文件
- **THEN** 格式步骤失败并列出该文件，后续步骤不再执行

#### Scenario: Broken spec

- **WHEN** 某个 change 的 requirement 缺少 scenario
- **THEN** `openspec validate --all --strict` 失败，PR 不可合并

#### Scenario: Cross-platform break

- **WHEN** 某段代码只在 Linux 编译通过（例如误用 `syscall` 常量）
- **THEN** Windows 或 macOS 的交叉编译步骤失败

### Requirement: Opt-in corpus regression

依赖下载数据的语料回归 SHALL 只在手动触发时运行：

- 该任务 SHALL 仅在 `workflow_dispatch` 事件下启动。
- 任务 SHALL 先 `maps-fetch` 下载 `de_ancient` 与 `de_dust2` 几何，再运行 `strikeye-cli corpus --geometry-dir`。
- 指标差异 SHALL 让任务失败，不得只在日志里提示。

#### Scenario: Manual corpus run

- **WHEN** 维护者手动触发工作流并选择语料任务
- **THEN** 任务下载几何、跑两场 fixture 并与黄金快照比较，任何差异都使任务失败

#### Scenario: Ordinary pull request

- **WHEN** 一个普通 PR 触发 CI
- **THEN** 语料任务被跳过，PR 不依赖外网
