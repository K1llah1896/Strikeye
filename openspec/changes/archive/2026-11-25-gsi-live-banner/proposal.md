# Proposal: 资料库 GSI 直播条

## Why

`gsi-live` 已经把"正在进行的比赛"解析成状态并镜像到 `cache/gsi_latest.json`，但这份数据只有 CLI 与脚本能读：用户打开 UI 时看不到直播中的比分与人数，也不知道镜像是新鲜还是过期。报告 §3 的 GSI 行虽然从 ✘ 改成了 ✔，界面侧仍是空白。

## What Changes

- 资料库页顶部新增「GSI 直播」状态条：地图、回合、比分、阶段、炸弹、存活人数。
- 镜像新鲜度 15 秒；过期或缺失时整条不渲染，避免用旧数据误导。
- 每 2 秒最多重读一次镜像文件，读失败静默降级。
- 保持只读：UI 不写镜像、不监听端口，写入仍由 `strikeye-cli gsi` 负责。

## Impact

- Affected specs: `gsi-live`
- Affected code: `internal/ui/views/library`（`live.go`、`live_test.go`、`library.go`）
