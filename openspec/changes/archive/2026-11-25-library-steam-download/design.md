# 设计

## 决策

**为什么让用户粘贴 Cookie 而不是做 Steam 登录**：OpenID 登录需要在浏览器里走三段重定向并回跳本地端口，还要处理 2FA；对"我只需要拉自己最近几场"的场景，粘贴一次 Cookie 成本更低。CLI 已经是这个模型，桌面端保持一致。

**为什么 Cookie 存 `cache/` 而不是 `config.json`**：`config.json` 会被备份（设置页有备份入口）、会被用户分享排障；Cookie 是会话凭据，泄露等于账号临时被接管。放进 `cache/` 且 0600，语义上"可丢弃"。

**为什么默认只下 3 场**：`media/imports` 会被 watcher 自动解析，一次拉 20 场会让解析队列堵住且磁盘暴涨。面板里保留 `steamLimit` 字段以便后续加输入框，默认 3。

**为什么下载后触发 `scanAndRefresh`**：下载目录就是 watcher 的 ImportDir，但扫描是周期性的；不主动扫会让用户以为"下载没生效"。

## 风险

- Cookie 过期后 Valve 返回 `g_steamID = false`，面板显示 "not signed in (cookie expired)"，需要用户重新粘贴。
- 下载是串行的且任何一场失败只记录不中断（`Failed` 列表），避免一场 404 让整批失败。
