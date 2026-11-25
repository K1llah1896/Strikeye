# 设计

## 决策

**为什么导出全部格式而不是弹窗选格式**：卡片上的按钮是"一键拿到可分享的产物"，四份文件加起来通常 < 1MB，比让用户先做一次格式选择更快。CLI 保留 `--format` 精细控制。

**为什么用 `rundll32 url.dll,FileProtocolHandler`（Windows）**：`cmd /c start "" "steam://…"` 需要处理双引号与路径转义，`rundll32` 直接吃 URL，且不会留下等待中的 shell 窗口。

**为什么把平台命令拆成 `Command(goos, url)`**：`exec.Command` 在测试里真的会拉起进程；把 argv 映射抽成纯函数后可以断言三平台行为而不启动任何东西。

**为什么空路径要提前报错**：`playback.URL("")` 会返回一个"合法但指向空"的 URL，交给处理器会静默失败或打开 CS2 停在主菜单。校验放在 `openDemoInCS2`，而不是 `playback.URL`，因为 CLI 的 `play` 已经对 demo 行做了 DB 校验。

## 风险

- 跨平台 `steam://` 需要本机装了 Steam；失败时只会在状态栏提示，不做安装检测（Steam 安装路径探测成本高、收益低）。
- 导出同步执行：一张比赛（4 表 + HTML + XLSX）实测 < 200ms，暂不适配异步；若后续加入大图或全部比赛批量导出，需要移到 goroutine。
