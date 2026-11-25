# Spec Delta: config-and-paths

## ADDED Requirements

### Requirement: Probe discovers bundled tools

工具链探测 SHALL 在 DataRoot 的 `tools/` 目录中发现随包解压的二进制，使设置页与渲染链路无需用户填写路径：

- `ProbeOptions.DataRoot` 非空时 SHALL 检查 `<DataRoot>/tools/<tool>/` 下的候选路径：`<exe>`、`bin/<exe>`、`*/<exe>`、`*/bin/<exe>`、`*/*/bin/<exe>`；
- 候选顺序 SHALL 位于系统 PATH 之前，使包内版本优先；
- 已显式配置且文件存在的路径 SHALL 优先于所有候选；
- 探测 SHALL 使用 glob 而非目录遍历，且不得因目录不存在而报错。

#### Scenario: Staged ffmpeg wins over PATH

- **WHEN** `<DataRoot>/tools/ffmpeg/ffmpeg-9.1-essentials_build/bin/ffmpeg.exe` 存在
- **THEN** 探测结果 `ffmpeg.path` 指向该文件，即使系统 PATH 里也有 ffmpeg

#### Scenario: Staged HLAE brings its hook

- **WHEN** `<DataRoot>/tools/hlae/HLAE.exe` 与 `x64/AfxHookSource2.dll` 存在
- **THEN** `hlae` 与 `afx_hook_source2` 都报告可用

#### Scenario: Configured path still wins

- **WHEN** 用户显式配置了 `paths.ffmpeg_exe` 且该文件存在
- **THEN** 探测返回该路径，不切换为内置版本

#### Scenario: Missing tools directory is not an error

- **WHEN** DataRoot 下没有 `tools/` 目录
- **THEN** 探测正常返回，其余字段与未加该能力前一致
