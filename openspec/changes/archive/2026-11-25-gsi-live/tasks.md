# Tasks

## 1. Ingest

- [x] 1.1 `Parse` + `flexInt`/`flexFloat` 容错
- [x] 1.2 `playersFrom` 合并 `player` 与 `allplayers`
- [x] 1.3 `Store`（Latest/Subscribe/OnUpdate）
- [x] 1.4 `Handler`（POST、token、1MB 限制、400/401/405）
- [x] 1.5 `Config`（Valve KeyValues、可选 auth）

## 2. CLI

- [x] 2.1 `gsi` 子命令与 usage
- [x] 2.2 配置落盘 + 监听 + ndjson 摘要 + 状态镜像
- [x] 2.3 `--once` 超时与 SIGINT 退出

## 3. Verification

- [x] 3.1 解析单测（字符串数值、武器弹药、死亡选手）
- [x] 3.2 Handler 单测（错误 token、坏载荷、GET、订阅与回调）
- [x] 3.3 配置单测（KeyValues 结构、token 块、花括号平衡）
- [x] 3.4 CLI 端到端单测（配置、POST、镜像文件、摘要、once 超时）
- [x] 3.5 真实 `gsi + curl` 冒烟：de_ancient 9 回合 5-4、炸弹已安放、存活 2/0
- [x] 3.6 `go test ./...`、`openspec validate gsi-live --strict`
