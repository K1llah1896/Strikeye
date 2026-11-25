## Context

GCPD 页面的结构（2026-11 现状）：比赛列表在 `td.val_left` 单元格里，每个单元格是一张嵌套小表，第一行是指向 `https://replayN.valve.net/730/<matchID>_<outcomeID>.dem.bz2` 的链接，第二行是 `YYYY-MM-DD HH:MM:SS GMT` 时间。没有 demo 链接的行表示 Valve 已过期该录像，且后续行通常也没有，可以直接停止。

翻页靠页面里的 `g_sGcContinueToken` 与 `g_sessionID`，以 `ajax=1&continue_token=...&sessionid=...` 请求后续片段，返回 JSON 的 `html` 字段。

## Goals / Non-Goals

**Goals**

- 一条命令把"最近的官方 demo"抓进资料库并自动解析。
- 解析只依赖 HTTP 与 HTML，不引入浏览器或 JS 引擎。
- 失败可解释：无 Cookie、Cookie 过期、无 demo 链接、下载空体各有独立错误。

**Non-Goals**

- 不做 Steam 账号密码登录（需要 RSA 公钥 + 2FA + 设备确认，属于凭据管理）。
- 不做 FACEIT/5E/ESEA 的私有 API（各自要 OAuth/AppKey，且条款各异）。
- 不做后台定时抓取：默认动作由用户显式触发。

## Decisions

1. **正则而非 DOM**：只需要 demo URL 与时间两处；单元格是嵌套表格，按 `</td>` 切分会截断，改为"定位 URL 后向后再取 2400 字符窗口找时间"，对页面小改动更耐受。
2. **URL 校验放宽到 `/730/<match>_<outcome>.dem.bz2` 路径**：host 不是契约（存在镜像/代理），matchID 与 outcomeID 才是。
3. **Cookie 只经内存传递**：支持原始头与 Netscape 导出两种格式，`ReadCookieFile` 过滤非 Steam 域。
4. **自动去重**：`ParsePage` 用 matchID 去重（同一场可能同时出现在多个 tab），`Download` 发现目标文件非空即复用。
5. **默认落到 `media/imports`**：与 `demo-source-expansion` 的归档导入目录一致，下载完即被 watcher 接管。
6. **`MaxPages=4`**：一页约 8 场，4 页足够覆盖"最近一批"，同时避免翻页风暴。
7. **`STRIKEYE_GCPD_BASE` 环境变量**：只用于测试把 BaseURL 指到 httptest，不对外宣传。

## Risks / Trade-offs

- Valve 改版会破坏解析：已有 fixture 测试，改版时改一个正则即可。
- Cookie 属于敏感凭据：不落盘、不打日志；命令行直接传 Cookie 会进 shell 历史，文档建议用文件。
- 下载体可能很大（数百 MB）：用 `io.Copy` 流式写 `.part`，不做内存缓冲。
