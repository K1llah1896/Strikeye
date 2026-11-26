# Spec Delta

## MODIFIED Requirements

### Requirement: Release package size budget

发布包 SHALL 保持“所有核心能力随包提供”的前提下压缩体积，并 SHALL 由可复现的打包步骤保证：

- 可执行文件 SHALL 以 `-ldflags "-s -w"` 链接，且 SHALL 只有一个（见 `native-ui-shell-and-guard` 的单文件分派要求）；
- 打包 SHALL 对可执行文件用 `zip -9`、对已压缩的边车用 `zip -0`，并 SHALL 在打包后校验 zip 完整性；
- 内嵌 CJK 字体 SHALL 为子集，且 SHALL 由 `scripts/subset-fonts.sh` 可从上游字体重建；
- 新增 UI 文案使用的中日韩与标点字符 SHALL 在字体子集覆盖测试中校验，缺字时该测试 SHALL 失败。

#### Scenario: Release zip stays within budget

- **WHEN** 发布流水线构建 `strikeye-windows-amd64.zip`
- **THEN** 压缩包包含 `strikeye.exe`、`map_geometry.zip`、`ffmpeg-windows-amd64.tar.zst`、`hlae-windows-amd64.zip`，且总大小不超过 300 MB

#### Scenario: Subset fonts cover new UI text

- **WHEN** 新增的界面字符串包含子集里没有的汉字或标点
- **THEN** `go test ./assets/fonts/` 失败并指出缺失的码位
