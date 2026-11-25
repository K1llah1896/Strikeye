# Tasks

## 1. Parser

- [x] 1.1 `PlayerPurchase` + `WeaponPrice` / `IsDefaultLoadout` / `KnifeVariant`
- [x] 1.2 `RoundWindow.Purchases` 与 `RoundTracker.RecordPurchase`（买枪窗口裁剪、`freezeEnded` 标记）
- [x] 1.3 `ItemPickup` 订阅、冻结期后按 `time_sec` 折算并落 `storage.MatchPurchase`

## 2. Storage

- [x] 2.1 迁移 v8 `match_purchases` + 索引
- [x] 2.2 `ReplaceMatchPurchases` / `ListMatchPurchases` / `PurchaseTotals`

## 3. Surfaces

- [x] 3.1 CLI `buys`（按回合→玩家聚合、总额、均值）
- [x] 3.2 经济页回合抽屉显示每笔购买与花费

## 4. Verification

- [x] 4.1 价格表/默认装备/刀皮判定单测
- [x] 4.2 买枪窗口单测（回合开始前、冻结期后均被拒）
- [x] 4.3 storage 替换/排序/按 demo 隔离/汇总单测
- [x] 4.4 CLI 聚合与 `--round` 过滤单测
- [x] 4.5 真实 demo 重解析后 `buys` 输出与库内行数一致
- [x] 4.6 `go test ./...`、`openspec validate purchase-events --strict`
