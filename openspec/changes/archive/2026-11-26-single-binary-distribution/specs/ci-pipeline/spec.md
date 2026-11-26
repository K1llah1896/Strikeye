# Spec Delta

## MODIFIED Requirements

### Requirement: Release packaging keeps the package small

发布流水线 SHALL 在打包阶段执行体积相关的固定步骤，使包体不随依赖漂移而无记录地增长：

- 编译 SHALL 使用 `-trimpath` 与 `-ldflags "-s -w"`；
- 发布包 SHALL 只含一个可执行文件 `strikeye.exe`（桌面外壳 + 代理 + 命令行三入口合并），SHALL NOT 再打包 `strikeye-agent.exe` 或 `strikeye-cli.exe`；
- 打包 SHALL 分两组写入：可执行文件用 `zip -9`，已压缩边车用 `zip -0`；
- `assets-pack` SHALL 在编译前生成几何边车与工具边车，且工具边车的上游版本 SHALL 在 `internal/toolbundle` 中钉定（而非 `latest` 标签）；
- 打包后 SHALL 执行 zip 完整性校验并输出 `SHA256SUMS.txt`。

#### Scenario: Release build reports a verified archive

- **WHEN** 打 tag 触发发布流水线
- **THEN** 生成的 `strikeye-windows-amd64.zip` 通过完整性校验，`SHA256SUMS.txt` 与之一同发布，且压缩包内只有一个可执行文件

#### Scenario: Pinned tool upstream

- **WHEN** 构建 `ffmpeg-windows-amd64.tar.zst`
- **THEN** 上游压缩包来自 `internal/toolbundle` 中钉定的直链，解出的 `ffmpeg.exe`/`ffprobe.exe` 在打包机与被分发机器上一致

#### Scenario: Development shims stay buildable

- **WHEN** 在仓库内执行 `go build ./...`
- **THEN** `cmd/strikeye`、`cmd/strikeye-agent`、`cmd/strikeye-cli` 都能编译，供 `go run` 与 CI 脚本使用，但它们不进入发布包
