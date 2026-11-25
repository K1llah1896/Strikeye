# match-library-and-analysis-ui

## ADDED Requirements

### Requirement: Card export and CS2 hand-off

资料库的比赛卡片 SHALL 提供导出与在 CS2 中打开两个动作：

- 「导出数据」SHALL 写出 CSV、JSON、XLSX 与 HTML 四种产物到 `<DataRoot>/media/exports`，并在状态栏显示文件数与目录。
- 导出 SHALL 复用 `internal/export` 的比赛包，不得复制列定义。
- 「用 CS2 打开」SHALL 生成 `steam://rungameid/730//+playdemo <转义后的绝对路径>` 并交给系统默认处理器。
- 平台命令 SHALL 可映射：Windows `rundll32 url.dll,FileProtocolHandler`、macOS `open`、其他 `xdg-open`。
- demo 行的文件路径为空时 SHALL 直接报错，不得调用处理器。
- 处理器返回错误时 SHALL 原样上报到状态栏。

#### Scenario: Export from a card

- **WHEN** 用户点击某场比赛卡片的「导出数据」
- **THEN** `media/exports` 下出现 CSV/JSON/HTML/XLSX，状态栏显示导出文件数与目录

#### Scenario: Open in CS2

- **WHEN** 用户点击「用 CS2 打开」且 demo 有绝对路径
- **THEN** 生成带转义路径的 `steam://` 链接并交给默认处理器

#### Scenario: Demo without a path

- **WHEN** demo 行的文件路径为空
- **THEN** 命令报错 "demo has no file path"，处理器不被调用
