# native-ui-shell-and-guard Specification

## Purpose

Defines the standalone MyGo Native UI desktop shell shipped as a single executable that also dispatches to the background agent and the command line, the startup render-mutex guard dialog, agent lifecycle coordination, dark esports theme, and the Settings & Tasks management view.

## Requirements

### Requirement: Render mutex guard dialog on startup
When `strikeye.exe` starts while `Local\StrikeyeCS2RenderActiveMutex` is held, the application SHALL NOT open the main UI window and SHALL instead display a lightweight native Guard Dialog polling `render.status` over IPC.

#### Scenario: Continue background rendering
- **WHEN** the user clicks "Continue Background Rendering" in the Guard Dialog
- **THEN** `strikeye.exe` exits immediately without disturbing the active HLAE/CS2 render session

#### Scenario: Safely abort rendering and open UI
- **WHEN** the user clicks "Safely Abort & Open Strikeye" in the Guard Dialog
- **THEN** `strikeye.exe` sends `render.abort_and_cleanup` over IPC, waits for `Local\StrikeyeCS2RenderActiveMutex` to be released, acquires `Local\StrikeyeUISingletonMutex`, and opens the main UI window

### Requirement: UI singleton, agent auto-launch, and render-now handoff
When no render mutex is active, `strikeye.exe` SHALL acquire `Local\StrikeyeUISingletonMutex`, automatically launch `strikeye-agent` in the background if `agent.ping` fails, and render the main MyGo Native UI window without any WebView, HTML, or JS runtime.

#### Scenario: User triggers immediate render from UI
- **WHEN** the user clicks "Start Render Now" in the UI and confirms the modal prompt
- **THEN** `strikeye.exe` sends `render.start_now` to `strikeye-agent` and terminates cleanly so the agent can acquire the render mutex

### Requirement: Main navigation shell and themed controls
The main window SHALL bind the active Strikeye theme palette to the UI context so every native control inherits themed surfaces, apply `Rajdhani` to numeric metrics and `Noto Sans SC` to CJK text, and provide a two-tier left navigation rail covering all five views.

#### Scenario: Default palette applies to native controls
- **THEN** with the default dark esports preset (`#0F141C` background, `#171E29` cards, `#F59E0B` / `#38BDF8` accents) inputs, selects, checkboxes, sliders, and scrollbars render with themed surfaces

#### Scenario: Navigation rail reaches every view
- **THEN** the two-tier rail exposes Match Library, Match Analysis, 2D Replay, Highlights, and Settings & Tasks

### Requirement: Live agent status bar and render-now modal
The top status bar SHALL display live agent state, active parse progress, and queued task counts via IPC event subscription, and render-now confirmation SHALL use a centered modal overlay.

#### Scenario: Live task status update
- **WHEN** `strikeye-agent` publishes a demo parse or render task state change event
- **THEN** the top status bar updates its status indicator and pending task count without requiring a manual refresh

#### Scenario: Render-now confirmation uses centered modal overlay
- **WHEN** the user clicks "Start Render Now" in the UI
- **THEN** a centered modal overlay appears above the workspace without shifting or compressing the main layout flow

### Requirement: Settings & Tasks view and agent IPC extensions
The `Settings & Tasks` view SHALL provide environment probing (`cs2.exe`, `HLAE.exe`, `ffmpeg.exe`, `ffprobe.exe`), directory and storage management (backup DB & clean invalid cache), render policy/quality configuration, and a live `render_tasks` queue with priority, cancel, and retry controls backed by `render.cancel`, `render.retry`, `config.reload`, and `montage.export` IPC handlers.

#### Scenario: Retry failed task and reload config
- **WHEN** the user clicks retry on a failed render task or saves updated settings
- **THEN** the UI invokes `render.retry` or saves config and calls `config.reload` over IPC, updating the task queue or environment status immediately

### Requirement: Distinct borderless secondary tab styling
Secondary workspace tabs (`TabButton`) SHALL use a borderless bottom-indicator visual style (active bottom accent bar and crisp text contrast) so they are visually distinct from bordered action buttons (`ActionButton`, `AccentButton`).

#### Scenario: Active vs inactive secondary tab
- **WHEN** a user views a multi-tab workspace such as `Match Analysis` or `Highlights`
- **THEN** the active tab displays a bottom accent indicator without a heavy outer action-button border, while inactive tabs remain muted and flat

### Requirement: OS-aware tool labels and compact Settings & Tasks layout
The `Settings & Tasks` view SHALL display OS-appropriate binary labels (omitting `.exe` suffixes for `ffmpeg` and `ffprobe` on non-Windows platforms) and keep the watch-directory editor compact so the `render_tasks` queue section remains accessible without excessive vertical scrolling.

#### Scenario: Viewing Settings & Tasks on Linux
- **WHEN** the user opens `Settings & Tasks` on Linux with `/usr/bin/ffmpeg` detected
- **THEN** the tool row displays `ffmpeg` and `ffprobe` without `.exe` suffixes and the task queue section sits higher in the viewport

### Requirement: Unclipped Settings & Tasks queue visibility at default window height
The `Settings & Tasks` view SHALL use compact vertical card padding and spacing so `4. 后台渲染任务队列` and its empty-state prompt remain unclipped inside the `1360x840` main window.

#### Scenario: Viewing Settings & Tasks at 1360x840
- **WHEN** the user opens `Settings & Tasks` at the default `1360x840` window size with an empty render queue
- **THEN** the `4. 后台渲染任务队列` header and its empty-state message fit cleanly above the bottom edge without half-line clipping

### Requirement: Customizable theme presets, accent colors, and background images
The UI theme system SHALL support switching between built-in theme presets (including dark esports and light/alternative presets), overriding the primary accent color via preset swatches or hex input, and rendering a user-selected local background image behind the main workspace with a configurable readability overlay and card opacity.

#### Scenario: Applying a custom accent color and background image
- **WHEN** the user selects a theme preset, enters a valid hex accent color, and selects a local PNG/JPG background image
- **THEN** the main shell renders the cached background image with the configured overlay tint, applies the accent color to active indicators and primary buttons, and updates all views immediately without restarting

#### Scenario: Invalid or missing background image fallback
- **WHEN** the configured background image path does not exist or fails to decode
- **THEN** the shell falls back cleanly to the solid theme base background color without crashing or blocking rendering

### Requirement: Appearance settings in Settings & Tasks
The `Settings & Tasks` view SHALL include an Appearance & Theme configuration section allowing users to choose a theme preset, pick or input an accent color, specify or clear a background image path, adjust overlay and card opacity sliders, and persist the configuration.

#### Scenario: Saving appearance configuration
- **WHEN** the user adjusts theme settings in `Settings & Tasks` and clicks save
- **THEN** the active UI theme updates immediately and the settings are written to `config.json`

### Requirement: Reusable progress bar and non-blocking async view loading
The UI shell SHALL provide a reusable progress bar primitive supporting determinate (`0.0–1.0`) and indeterminate loading states, and SHALL execute heavy demo selection transitions (`Match Analysis`, `Spectator`, `Highlights`) asynchronously so the UI remains responsive and displays a loading progress bar until data is ready.

#### Scenario: Navigating to a heavy view displays a loading progress bar
- **WHEN** the user clicks to open `Match Analysis` for a parsed demo
- **THEN** the workspace switches immediately and displays a loading progress bar with stage text until SQLite queries and initial canvas setup complete

### Requirement: Headless render smoke tests

视图 SHALL 具备无头渲染回归测试：

- `internal/ui/renderutil` SHALL 提供 `Probe` 与 `AssertRendered`，用 `ui.Render` 在 1280×800 渲染并按 4px 采样统计不同颜色数。
- 资料库、设置、分析、精彩回放、观战五个视图 SHALL 各至少有一个渲染测试，渲染结果颜色数低于阈值时 SHALL 失败。
- 分析视图 SHALL 逐一渲染全部标签页（总览/经济/表现/雷达/2D 回放/热力图/区域控制/队伍视角）。
- 测试数据 SHALL 由临时 SQLite 播种，不依赖真实 demo 文件。

#### Scenario: Blank frame detected

- **WHEN** 某视图因 nil 指针或空数据只画出背景
- **THEN** 渲染测试因颜色数不足失败，而不是通过

#### Scenario: Panel branches covered

- **WHEN** 下载面板、API Key 面板或 SQL 结果表格处于展开状态
- **THEN** 渲染测试覆盖这些分支并保持通过

### Requirement: Settings installs tools from the package

设置页的工具安装入口 SHALL 默认从随包的边车安装，而不是从网络下载：

- 缺失 HLAE/FFmpeg 时按钮文案 SHALL 为“从内置包安装”；
- 点击后 SHALL 只在本地解压（无网络请求），并在完成后写入配置路径、重新探测、通知 agent 重载配置；
- 边车缺失时 SHALL 显示带具体文件名的提示与源码构建命令（`strikeye-cli assets-pack --tools-out <目录>`），而不是静默失败；
- “地图数据”卡片的一键安装 SHALL 同时安装地图数据与工具，并汇总两者结果。

#### Scenario: Offline install from the package

- **WHEN** 用户在设置页点击「从内置包安装 FFmpeg」，且边车存在
- **THEN** 无网络请求，`paths.ffmpeg_exe` 被写入内置路径，状态显示安装完成

#### Scenario: Missing sidecar explains itself

- **WHEN** 边车不存在时点击该按钮
- **THEN** 状态区显示“本安装包未附带 ffmpeg（期望文件名 ffmpeg-windows-amd64.zip）”及 `assets-pack --tools-out` 提示

#### Scenario: One-click map data also stages tools

- **WHEN** 用户在「地图数据」卡片点击一键安装
- **THEN** 摘要同时报告导航网格/callout/几何数量与工具安装结果

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
