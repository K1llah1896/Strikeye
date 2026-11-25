# STRIKEYE

### SEE THE ROUND. OWN THE MOMENT.
**Beyond the Demo. Beyond the Score.**

Strikeye 是一款面向 Counter-Strike 2 玩家打造的本地化比赛观战与分析平台。

以 Demo 为核心，Strikeye 将原始比赛记录转化为结构化、可视化、可交互的比赛体验，让玩家摆脱繁琐的 Demo 管理与回放操作，轻松探索每一场比赛中的关键回合、选手表现与精彩瞬间。

通过比赛管理、独立观战、数据分析与精彩片段管理，Strikeye 让每一场比赛都变得可观看、可理解、可探索、可留存。

---

## The Story Behind Strikeye

在 Counter-Strike 的世界里，比分记录着胜负，但真正决定比赛走向的，往往是那些容易被忽略的瞬间。

一次精准的首杀、一次绝境中的 Clutch、一颗改变战局的闪光弹，或是一位选手在关键回合中的精彩发挥。这些瞬间构成了比赛真正的价值，却常常被埋藏在冗长的 Demo 文件和繁琐的回放操作之中。

**Strikeye 因此而生。**

我们希望改变玩家观看和理解比赛的方式，让每一场比赛都能被轻松回顾，让每一个关键回合都能被快速找到，让每一位选手的表现都能被看见。

Strikeye 不希望 Demo 只是被下载、保存和遗忘的比赛文件，而希望它成为一份可以反复探索的比赛档案。

无论是复盘一次失误、研究一名选手的表现，还是重新欣赏一场精彩的残局，我们都希望让这些原本稍纵即逝的瞬间，成为随时可以回顾和分享的比赛记忆。

因为比赛不应该只留下一个比分。

**每一回合都有值得回看的故事，每一个瞬间都有值得记录的价值。**

---

## What Strikeye Does

Strikeye 围绕比赛的完整生命周期，构建统一的 Demo 管理、观战与分析体验。

### Match Library — 比赛管理

将分散的 Demo 文件转化为有序的本地比赛资料库。

集中管理比赛记录，快速检索比赛信息、地图、回合与选手，让每一场比赛都能够被轻松找到和重新访问。

### Spectator — 独立观战

让 Demo 回放回归直观和高效。

自由选择比赛、回合及选手视角，快速定位关键时刻，探索比赛进程，重新观看每一个值得关注的细节。

### Match Analysis — 比赛分析

让数据帮助玩家理解比赛。

通过 Rating、K/D、ADR、KAST 等表现指标，以及击杀、首杀、残局和其他关键事件，深入了解选手表现与比赛走势，让每一次复盘都有据可循。

### Highlights — 精彩回放

让精彩瞬间不再被埋没。

自动发现比赛中的高光时刻，支持筛选、收藏、回放与剪辑，将一场比赛中最值得记住的瞬间整理成属于自己的精彩集锦。

### 能力清单（截至 2026-11）

资料库：18 类来源识别、压缩包自动解压、`.info` 旁挂元数据、GCPD 官方 demo 下载、share code 导入、全文检索（含中文子串）、系列赛分组、选手档案与标签、封禁快照、头像、GSI 直播条、SQL 控制台、地图数据（碰撞几何 / 导航网格 / callout）一键下载。

分析：Rating / ADR / KAST / K-D / 首杀 / 交易 / 残局、TTK 与 TTD（含开镜视锥与追踪遮挡）、逐 kill 击杀上下文（距离/视线/穿烟/背光/开镜夹角，可落库与导出）、经济与购买时序、区域控制与击杀区位、默认站位热力图、navmesh 地图控制曲线（含烟雾/火焰遮蔽）、道具 line-up 库与收益归因。

回放与产出：32Hz 2D 回放（弹道/死亡标记/C4 爆炸范围/击杀信息条/上下层雷达/自由笔刷与区域标记/语音时间轴）、多格式导出（CSV/JSON/XLSX/HTML）、多轨音频混音、HLAE 录制与集锦合成。

完整的差分调研与依据见 [`docs/research/2026-11-25-cs2-oss-gap-analysis.md`](docs/research/2026-11-25-cs2-oss-gap-analysis.md)，指标口径见 [`docs/metrics.md`](docs/metrics.md)。

---

## Our Philosophy

### SEE THE ROUND.

看见每一回合。

不局限于最终比分，而是深入比赛过程。观察选手的每一次行动，理解关键回合的决策，发现数据背后的表现与价值。

### OWN THE MOMENT.

掌握每一个瞬间。

不再花费时间寻找 Demo 中的关键时刻。快速定位、反复回放、记录与分享，让每一个精彩瞬间都尽在掌握。

---

## Our Vision

我们希望 Strikeye 成为 CS2 玩家自己的比赛档案与观战中心。

从单场比赛的快速回放，到选手表现的深入分析；从一次精彩击杀的记录，到跨越多场比赛的个人集锦。

让 Demo 不再只是比赛结束后留下的文件，而成为可以持续探索、积累和回顾的数字化比赛记忆。

**Strikeye，让每一回合清晰可见，让每一个精彩瞬间尽在掌握。**

---

## Build & Release

本地构建（可交叉编译出 Windows 可执行文件）：

```bash
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath \
  -ldflags "-X strikeye/internal/version.AppVersion=0.2.0 -H windowsgui" \
  -o dist/strikeye.exe ./cmd/strikeye
```

自动发布：push 版本 tag 即触发 `.github/workflows/release.yml`。

```bash
git tag v0.1.0
git push origin v0.1.0
```

工作流会执行 `go vet` / `go test`，交叉编译三个可执行文件，打包 `strikeye-windows-amd64.zip`（内含 `strikeye.exe` / `strikeye-agent.exe` / `strikeye-cli.exe`）与 `SHA256SUMS.txt`，并创建对应的 GitHub Release。tag 中的版本号会通过 `-X strikeye/internal/version.AppVersion` 注入，用于应用内更新比较。

带 `-` 的 tag（如 `v0.2.0-rc1`）会发布为 prerelease，不会顶掉 `/releases/latest`；应用内更新（`internal/updater`）读取的正是该地址，因此正式更新请使用 `vX.Y.Z` 形式的 tag。

## 开发校验

每次 push / PR 都会跑 `.github/workflows/ci.yml`：格式（`gofmt -l`）、`go vet`、`go test ./...`、三平台交叉编译（linux/amd64、darwin/arm64、windows/amd64）与 `openspec validate --all --strict`（CLI 钉在 1.15.0）。本地等价命令：

```bash
gofmt -l ./cmd ./internal        # 必须为空
go vet ./... && go test ./... -count=1
GOOS=darwin GOARCH=arm64 go build ./...
openspec validate --all --strict
```

指标回归用黄金快照，需先下载地图几何（TTD 依赖 `.tri` 几何），这条任务在 CI 里是手动触发（`workflow_dispatch`），避免 PR 依赖外网：

```bash
go build -o bin/strikeye-cli ./cmd/strikeye-cli
./bin/strikeye-cli maps-fetch --data-root .ci-data --map de_ancient --map de_dust2
./bin/strikeye-cli corpus --geometry-dir .ci-data/cache/map_geometry
./bin/strikeye-cli corpus --update --geometry-dir dist/cache/map_geometry   # 口径变了才重生成
```

---

<sub>STRIKEYE — CS2 MATCH REPLAY & ANALYSIS PLATFORM</sub>

<sub>Built for players who see beyond the score.</sub>