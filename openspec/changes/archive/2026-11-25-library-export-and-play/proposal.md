# 资料库导出与「用 CS2 打开」

## 为什么

导出（CSV/JSON/XLSX/HTML）与 `steam://` 打开 demo 都只有 CLI 子命令。资料库里每张卡片已经有"查看分析""浏览精彩片段""进入观战"，但分析师想把这场的表格发给教练、或者直接进游戏看某个回合时，得切到终端敲命令。

## 变什么

1. 每张比赛卡片新增两个动作：
   - 「导出数据」：一次性写出 CSV（四张表）+ JSON + XLSX + 自包含 HTML 报告到 `<DataRoot>/media/exports`，状态栏回报文件数与目录。
   - 「用 CS2 打开」：把 `steam://rungameid/730//+playdemo <path>` 交给系统默认处理器（Windows `rundll32`、macOS `open`、Linux `xdg-open`）。
2. 新增 `internal/launcher`：平台 → 命令映射可单测，`Open` 负责启动并回收子进程。
3. 空路径的 demo 行直接报错，不把垃圾 URL 交给处理器。

## 影响

- capability：`match-library-and-analysis-ui`（ADDED「Card export and CS2 hand-off」）。
- 导出复用既有 `internal/export`，不新增格式；`media/exports` 已在 `.gitignore` 覆盖范围内（DataRoot 侧目录）。
