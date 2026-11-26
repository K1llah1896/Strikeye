# Spec Delta

## ADDED Requirements

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
