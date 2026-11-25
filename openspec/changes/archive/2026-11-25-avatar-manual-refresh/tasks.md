# Tasks

- [x] `View.refreshAvatars`（后台、2 分钟超时、统计新下载/已缓存）
- [x] 标题行「刷新头像」按钮 + 进行中徽章 + 结果文案
- [x] 修 `avatars.Client.download` nil HTTP 崩溃，`refreshAvatars` 改走 `avatars.NewClient()`
- [x] 测试：无选手不启动、假 profile 端到端取图并解码、零值 Client 不 panic
- [x] `go test ./...`、`go vet ./...`、`openspec validate --strict`
- [x] 报告 §8 更新
