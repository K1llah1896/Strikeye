# Spec Delta

## ADDED Requirements

### Requirement: Single-binary entry point dispatch

发布包 SHALL 只包含一个可执行文件，并在启动时按参数与程序名分派到桌面外壳、后台代理或命令行：

- 无参数或非分派参数 SHALL 进入桌面外壳；
- 首个参数为 `cli`（大小写不敏感）SHALL 进入命令行，并 SHALL 剥掉该选择子后再交给子命令解析；
- 首个参数为 `agent` SHALL 在当前进程内运行后台代理服务；
- 程序名（去掉 `.exe` 后）以 `-cli` 或 `-agent` 结尾时 SHALL 分别进入命令行与代理模式，使重命名副本与开发构建保持旧行为；
- 既有 `--agent` 标志 SHALL 继续可用；
- 分派判断 SHALL 由纯函数实现并可单独测试，不需要启动 UI 或写盘。

#### Scenario: Command line through the merged binary

- **WHEN** 运行 `strikeye cli export --data-root D --demo-id 1`
- **THEN** 执行与 0.4.0 的 `strikeye-cli export` 相同的导出，退出码与标准输出一致

#### Scenario: Agent mode in-process

- **WHEN** 运行 `strikeye agent --data-root D --pipe P`
- **THEN** 该进程以 IPC 服务身份运行，行为与 `strikeye-agent.exe` 相同

#### Scenario: Desktop shell unchanged

- **WHEN** 双击 `strikeye.exe` 或运行时不带分派参数
- **THEN** 打开原生 UI 外壳，并按既有守卫与单实例流程自举代理（自举子进程使用同一文件加 `--agent`）

#### Scenario: Renamed copy keeps the old entry point

- **WHEN** 把发布包中的 `strikeye.exe` 复制为 `strikeye-cli.exe` 并运行 `--version`
- **THEN** 输出命令行版本信息而不是打开桌面外壳

#### Scenario: CLI output survives the GUI subsystem

- **WHEN** 在 Windows 控制台中运行 `strikeye cli doctor`
- **THEN** 命令输出出现在该控制台（进程附着父控制台并重绑标准流），而不是静默丢弃

### Requirement: Merged binary replaces split executables on update

热更新 SHALL 清理被合并的旧可执行文件，并 SHALL NOT 影响仍带旧布局的更新包：

- 更新包包含 `strikeye` 或 `strikeye.exe` 且不含 `strikeye-agent`/`strikeye-cli`（含 `.exe`）时，目标目录中的这些旧文件及其 `.old` 备份 SHALL 被删除；
- 更新包自身包含旧名称时 SHALL 全部保留，使降级或过渡包可用。

#### Scenario: Upgrade from the split layout

- **WHEN** 在 0.4.0 的安装目录（三个可执行文件）上应用 0.5.0 的更新包
- **THEN** 目录中只留下 `strikeye.exe`，旧代理与 CLI 不再存在

#### Scenario: Transitional package keeps them

- **WHEN** 更新包同时包含 `strikeye.exe`、`strikeye-agent.exe` 与 `strikeye-cli.exe`
- **THEN** 三个文件都被安装，没有任何一个被删除
