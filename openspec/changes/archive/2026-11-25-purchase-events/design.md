## Context

demoinfocs 在 CS2 demo 上能稳定给出 `events.ItemPickup`（实测 dust2 19 回合 1436 次，含刀具与 C4）。它同时覆盖"买"和"捡"，因此需要窗口与时序过滤才能当成购买。

## Goals / Non-Goals

**Goals**

- 每回合能回答"谁、第几秒、买了什么、多少钱"。
- 与既有经济快照（装备值/花费/剩余）互相印证。
- 不改变回放缓存格式，不新增第三方依赖。

**Non-Goals**

- 不做逐件发票金额还原（退款、折扣、卖回）。
- 不做购买路线/站位（属 2D 回放范畴）。
- 不追加热身与刀局。

## Decisions

1. **窗口 = 回合开始 → 冻结期结束**：`RecordPurchase` 拒绝 `tick < StartTick` 与冻结期结束后的记录。冻结期结束前 `freezeEnded=false` 时不裁剪，避免"还不知道终点就把有效购买丢掉"。
2. **排除出生免费装备**：`Knife`、`C4`、`Glock-18`、`USP-S`、`P2000` 不算购买；刀皮名（含 `★`/`bayonet`/`karambit`）另用 `KnifeVariant` 排除。
3. **价格用静态买单价**：`WeaponPrice` 覆盖手枪/微冲/步枪/重武器/护甲/道具共 40 项。未知物品直接不记（宁缺毋错），而不是记 0 元。
4. **`time_sec` 相对冻结期结束，可为负**：正值表示冻结期结束后才补买，负值是买枪阶段的常态；第一回合冻结期可能远长于 20 秒（实测 dust2 round 1 从 tick 342 到 5995），所以不设下限。
5. **落库随解析一并替换**：`ReplaceMatchPurchases` 在解析结束时整场替换，保证重解析不产生重复行。

## Risks / Trade-offs

- `ItemPickup` 在部分第三方 demo 可能缺失 → 该场 `match_purchases` 为空，CLI 输出 `purchases: 0`，不影响其它分析。
- 价格表随 Valve 调价会过时 → 记录在 `docs/metrics.md` 与 spec 中，属可更新常量。
