# Tasks

- [x] `internal/voice.BuildActivity`（0.5s 合并）+ `FilterRange` + `BySpeaker`
- [x] 测试：合并/拆分/裁剪/分组顺序
- [x] 回放页「语音时间轴」卡片（字符条 + 时长 + 段数 + 无语音文案）
- [x] UI 测试：字符条宽度/裁剪、行排序与名字回退、截断
- [x] CLI `voice-activity --demo-id [--round]`
- [x] CLI `voice-decode --xuid`
- [x] 真实验证：合成捕获两个说话人 → 分段正确、round 过滤生效
- [x] `go test ./...`、`go vet ./...`、`openspec validate --strict`
- [x] 报告 §4.3 与 §8 更新
