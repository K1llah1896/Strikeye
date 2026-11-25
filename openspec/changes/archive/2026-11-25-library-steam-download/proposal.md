# 资料库内 Steam 下载面板

## 为什么

`strikeye-cli download` 能从 matchhistory 拉官方 demo，但 GUI 用户没有入口：他们要先打开终端、找到 Cookie、拼命令行，再把文件放进监听目录。下载本身的价值是"闭环到自动解析"，而闭环断在终端。

## 变什么

1. 资料库工具栏新增「从 Steam 下载」面板：
   - Cookie 输入框（粘贴浏览器 Cookie 请求头）+ 匹配类型下拉（`gcpd.Tabs` 五项）；
   - 「下载并导入」：拉列表、下载新 demo 到 `<DataRoot>/media/imports`，随后触发一次扫描刷新；
   - 状态栏回报"已下载 N 场（跳过 M，失败 K）到 <目录>"。
2. Cookie 落盘到 `<DataRoot>/cache/gcpd_cookie.txt`，权限 0600，不写入 `config.json`（避免配置备份或同步带走会话凭据）。
3. 下载在后台执行，`steamDownloadTimeout = 30 分钟`，进行中显示「下载中...」并阻止重复触发。
4. `STRIKEYE_GCPD_BASE` 环境变量与 CLI 一致地覆盖 BaseURL（便于镜像与测试）。

## 影响

- capability：`demo-download`（ADDED「In-app match history import」）。
- 复用 `internal/gcpd`，不复制解析与下载逻辑。
