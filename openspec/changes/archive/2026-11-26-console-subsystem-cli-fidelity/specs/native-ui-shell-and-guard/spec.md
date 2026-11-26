# Spec Delta

## MODIFIED Requirements

### Requirement: Single-binary entry point dispatch

发布包 SHALL 只包含一个可执行文件，并在启动时按参数与程序名分派到桌面外壳、后台代理或命令行：

- 无参数或非分派参数 SHALL 进入桌面外壳；
- 首个参数为 `cli`（大小写不敏感）SHALL 进入命令行，并 SHALL 剥掉该选择子后再交给子命令解析；
- 首个参数为 `agent` SHALL 在当前进程内运行后台代理服务；
- 程序名（去掉 `.exe` 后）以 `-cli` 或 `-agent` 结尾时 SHALL 分别进入命令行与代理模式，使重命名副本与开发构建保持旧行为；
- 既有 `--agent` 标志 SHALL 继续可用；
- 分派判断 SHALL 由纯函数实现并可单独测试，不需要启动 UI 或写盘；
- 可执行文件 SHALL 以控制台子系统链接，使命令行模式在 `cmd.exe` 与 PowerShell 中阻塞、流式输出并把退出码交回调用方；
- 桌面外壳启动时 SHALL 隐藏并释放仅属于本进程的控制台窗口（`GetConsoleProcessList` 返回 1），SHALL NOT 隐藏与真实终端共享的控制台（返回大于 1）；
- 进程在无控制台时 SHALL 尝试附着父控制台并重绑标准流，使以 GUI 子系统链接的自建版本仍能输出。

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

- **WHEN** 用 `-H windowsgui` 手工构建后在 Windows 控制台中运行 `strikeye cli doctor`
- **THEN** 命令输出出现在该控制台（进程附着父控制台并重绑标准流），而不是静默丢弃

#### Scenario: Script sees the CLI finish

- **WHEN** 在 `.bat` 或 PowerShell 脚本中运行 `strikeye cli export …`
- **THEN** 脚本等待命令结束，输出按顺序出现，`%ERRORLEVEL%`/`$LASTEXITCODE` 反映该命令的结果

#### Scenario: Double-click leaves no terminal behind

- **WHEN** 从资源管理器双击 `strikeye.exe` 打开桌面外壳
- **THEN** 为本进程创建的控制台窗口在渲染首帧前被隐藏并释放，不会留下空终端

#### Scenario: Terminal-launched shell keeps its console

- **WHEN** 在终端里运行 `strikeye` 打开桌面外壳
- **THEN** 该终端的窗口不被隐藏，其日志输出继续可见
