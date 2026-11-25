# Design: 黄金指标快照

## Context

端到端口径必须与用户看到的数字一致：`match_players` 表里的 ADR/KAST/Rating/TTK/TTD 就是 UI 与导出读的值，回放文件里的弹道/死亡数就是 2D 层画的东西。快照直接从解析产物读取，而不是重新实现一套计算。

## Decisions

1. **快照来自真实解析链路**：`BuildWithGeometry` 用 `parser.ParseAndPersist` 在临时 DataRoot 里跑一遍（`Force: true`），再从 SQLite 与回放缓存读取指标。这样任何"只在解析器里对、落库后错"的回归都能被抓到。
2. **临时 DataRoot 而非用户库**：默认 `os.MkdirTemp`，`--work-dir` 可复用（省去每次重建 DB），绝不写入 `dist/`。
3. **浮点容差 0.11**：指标已是 2 位小数，`ADR 106.29` 与 `106.3` 是同一次解析的结果；容差取 0.11 让"格式化差异"不失败，但 1 位以上小数变化必然失败。
4. **几何可选但推荐**：TTD 依赖 `.tri` 视线数据。快照记录 `ttd_samples`，传入 `--geometry-dir` 后 dust2 有 161、ancient 有 229 个样本；不传时恒为 0 并同样稳定，因此 CI 可以在没有几何的机器上继续跑，但推荐带几何。
5. **opt-in 测试**：两只 fixture 合计 700MB、解析约 30 秒（实测两图 44 秒含几何复制）。放进默认 `go test` 会让每次改动都等半分钟，所以用 `STRIKEYE_CORPUS=1` 显式开启；CI 里设置该变量即成为门禁。

## Risks

- 快照与 fixture 二进制绑定：`sha256` 进入快照，fixture 被替换时比对失败是期望行为，`--update` 重写即可。
- 指标口径变更（例如新增 KAST 规则）会让快照全部失败，需要人工确认后 `--update`：这正是快照存在的意义。
