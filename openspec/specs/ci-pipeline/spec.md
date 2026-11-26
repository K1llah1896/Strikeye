# ci-pipeline Specification

## Purpose

Define the automated gate that every push and pull request passes: formatting, vet, tests, cross-platform builds and OpenSpec validation, plus an opt-in corpus regression for changes that touch parse metrics. This capability exists so a metric or schema change cannot reach a release without the specs and the golden snapshots agreeing with it.

## Requirements

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

### Requirement: Release packaging keeps the package small

发布流水线 SHALL 在打包阶段执行体积相关的固定步骤，使包体不随依赖漂移而无记录地增长：

- 编译 SHALL 使用 `-trimpath` 与 `-ldflags "-s -w"`；
- 打包 SHALL 分两组写入：可执行文件用 `zip -9`，已压缩边车用 `zip -0`；
- `assets-pack` SHALL 在编译前生成几何边车与工具边车，且工具边车的上游版本 SHALL 在 `internal/toolbundle` 中钉定（而非 `latest` 标签）；
- 打包后 SHALL 执行 zip 完整性校验并输出 `SHA256SUMS.txt`。

#### Scenario: Release build reports a verified archive

- **WHEN** 打 tag 触发发布流水线
- **THEN** 生成的 `strikeye-windows-amd64.zip` 通过完整性校验，`SHA256SUMS.txt` 与之一同发布

#### Scenario: Pinned tool upstream

- **WHEN** 构建 `ffmpeg-windows-amd64.tar.zst`
- **THEN** 上游压缩包来自 `internal/toolbundle` 中钉定的直链，解出的 `ffmpeg.exe`/`ffprobe.exe` 在打包机与被分发机器上一致
