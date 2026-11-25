# 设计

## 决策

**为什么钉 openspec 版本**：`openspec validate --strict` 的规则随版本演进（历史上 2.x 收紧了 scenario 要求）。CI 用最新版会把"按 1.15 写的 spec"判红，而升级 spec 格式应该是一个显式的 change，不是被 CI 倒逼。故钉 `1.15.0`，升级时同步改 `OPENSPEC_VERSION` 并跑一遍全量校验。

**为什么语料回归不进 PR 门禁**：`corpus` 需要 `maps-fetch` 拉 `.tri` 几何（TTD 依赖它），且要跑两场 demo 各 16–25 秒。放在 `workflow_dispatch` 里手动触发，PR 保持 hermetic（无外网依赖、<5 分钟）。

**为什么先 gofmt 一遍仓库**：格式门禁的价值是"新代码不会带进噪音"，但如果仓库本身就不干净，门禁从第一天起就是红的，团队会学会忽略它。先清干净再上闸。

**为什么用 `gofmt -l` 而不是第三方 linter**：无额外依赖、行为稳定、与 `go vet` 互补。lint 规则集（staticcheck 等）等有明确收益再引入。

## 风险

- 编译三个平台只在 CI 做 build，不跑测试；跨平台的运行时差异（Windows 命名管道、HLAE）仍靠 `doctor` 与人工验证。
- gofmt 版本差异：CI 的 Go 版本由 `go.mod` 的 `go 1.27.4` 决定，本地与 CI 一致，不会互相打架。
