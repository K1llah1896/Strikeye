# demo-download

## ADDED Requirements

### Requirement: In-app match history import

资料库 SHALL 提供应用内的 match history 下载入口：

- 面板 SHALL 接受粘贴的 Cookie 请求头与匹配类型选择（`gcpd.Tabs` 中的五种）。
- Cookie SHALL 保存到 `<DataRoot>/cache/gcpd_cookie.txt`，权限 0600，不得写入 `config.json`。
- 下载 SHALL 在后台执行，进行中 SHALL 显示「下载中...」并阻止重复触发。
- 下载目标 SHALL 是 `<DataRoot>/media/imports`，已存在的非空文件 SHALL 跳过并计入 `skipped`。
- 单场失败 SHALL 记录到失败列表并继续处理后续场次，不得中断整批。
- 完成后 SHALL 在状态栏显示下载/跳过/失败数量与目录，并触发一次资料库扫描。
- `STRIKEYE_GCPD_BASE` SHALL 覆盖 Steam BaseURL，与 CLI 行为一致。

#### Scenario: Paste cookie and download

- **WHEN** 用户粘贴有效 Cookie、选择 `matchhistorypremier` 并点击「下载并导入」
- **THEN** 新 demo 出现在 `media/imports`，状态栏显示下载数量，随后的扫描把文件登记进资料库

#### Scenario: Expired cookie

- **WHEN** Cookie 已失效
- **THEN** 状态栏显示「下载失败: gcpd: not signed in (cookie expired)」，不产生半个文件

#### Scenario: Already downloaded

- **WHEN** 同一场 demo 已在 `media/imports`
- **THEN** 该场被计入 `skipped`，不重复下载
