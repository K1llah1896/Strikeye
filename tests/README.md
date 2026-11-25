# 测试语料（fixtures）

`tests/` 下的大体积 `.dem` 不进入 git（两份合计约 900 MB），黄金快照
（`tests/fixtures/golden/*.json`）进入 git，两者配对使用。

| 快照 | 比赛 | 地图 | 比分 | 回合 | 大小 | sha256 |
|---|---|---|---|---|---|---|
| `furia-vs-falcons-m1-ancient.json` | BLAST Rivals 2026 Season 2 半决赛（2026-11-14）FURIA vs Falcons | de_ancient | 16:14 | 30 | 548,068,016 | `f55460d91775de43d4d6ef890e857d4588e4d660a64cbbbc49fc5dca787ff128` |
| `furia-vs-falcons-m2-dust2.json` | 同上 | de_dust2 | Falcons 6:13 FURIA | 19 | 364,090,861 | `95f360d9fcff761248b504b06bf7f956f68313aa7e41ae5101b1d58610a3d399` |

## 怎么拿到这两份 demo

1. 职业比赛：HLTV 比赛页 → “Download demo”（需要 HLTV 账号登录），解压 `.rar` 得到 `.dem`，
   按上表文件名放进 `tests/`。
2. 自己的比赛：`strikeye-cli download --cookies <cookie> --tab mm`（写进 `media/imports`，
   被 watcher 自动导入），或直接把 `.dem` 丢进导入目录。

判重按内容 sha256，因此重复导入同一场不会产生第二行；文件名可以不同。

## 语料回归

```bash
# 用仓库根目录的边车几何（或 --geometry-dir 指定缓存目录）
go run ./cmd/strikeye-cli corpus                      # 对比黄金快照
go run ./cmd/strikeye-cli corpus --update             # 口径变了才重生成
STRIKEYE_CORPUS=1 go test ./internal/corpus/ -run TestFixtureCorpus -count=1
```

缺失的 demo 会被自动跳过（`fixture … not present`），因此 CI 与全新检出上看不到这两场；
本地只有一份 demo 时也只回归那一场。TTD 指标依赖碰撞几何，没有几何时快照里的 TTD 会变成 0。
