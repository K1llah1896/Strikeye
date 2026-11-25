## Why

报告 §3 的「经济 / 购买 / 道具事件」一行一直是 ◐：Strikeye 有冻结期装备值与花费，但没有"谁在第几秒买了什么"。缺了这一层，"对手 0:05 才买烟"这类时序判断做不了，也没法核对某回合到底是半起还是硬起。

## What Changes

- 解析单遍时订阅 `events.ItemPickup`，把买枪阶段（回合开始 → 冻结期结束）内的购买按 `(tick, 玩家, 物品, 价格)` 记进回合窗口。
- 迁移 v8 新增 `match_purchases`，随 demo 一起落库；`ReplaceMatchPurchases` / `ListMatchPurchases` / `PurchaseTotals`。
- CLI `buys --demo-id [--round]`：按回合 → 玩家聚合，给出每笔的 `time_sec`（相对冻结期结束）、玩家花费与回合总额。
- 回合详情抽屉在每个选手行下方追加「购买 $N · AK-47 -0:08 · 烟雾 -0:04」，并给出本回合笔数与总额。

## Impact

- 新增：`internal/parser/purchases.go`、`internal/storage/purchase_repo.go`、`cmd/strikeye-cli/buys.go`。
- 修改：`RoundWindow.Purchases`、`RoundTracker.RecordPurchase`（买枪窗口裁剪）、`single_pass.go`（事件订阅与落盘）、迁移、经济页抽屉。
- 价格表是静态买单价（`WeaponPrice`），不是发票金额；免费出生装备不计入。
