# 击杀上下文落库

## 为什么

`killctx` 只能每次重新算：逐 kill 的交火距离、视线遮挡、穿烟、开镜夹角、背身/致盲都停在内存里，"这场有多少枪穿烟""有哪些击杀是对手看不到的"这类问题只能在 JSON 里肉眼数。报告 §4.4 的"逐 kill 的 `kill_context` 明细供标注与查询"是这条链路上最后一块。

## 变什么

1. 迁移 v12 新增 `kill_context` 表（28 列：回合/tick/时间、双方槽位与名字、武器、爆头、距离、`visible`/`geometry_checked`/`smoke_blocked`/`optics_checked`、`aim_off_deg`/`in_fov`/`victim_sees_killer`、双方致盲与开镜等级、双方世界坐标），主键 `(demo_id, round_number, tick, victim_slot)`，两个索引（按 demo+回合、按 `smoke_blocked`）。
2. `storage.KillContextRepo`：`ReplaceMatchKillContext`（单事务重写）、`ListMatchKillContext`、`Totals`（聚合击杀数/平均距离/背身率/穿烟率）。
3. `killctx` 增加 `--store`，把计算结果写库；`killctx.Build` 的 `Kill` 增 `Weapon`（按 `PlayerSlot` 在 ±64 tick 内的最近一发推断）与 `KillerPos`/`VictimPos`。
4. 导出：比赛包新增 `match_<id>_kills.csv`、XLSX 第五张表 `kills`、JSON `kills` 数组。

## 影响

- capability：`kill-context`（ADDED「Stored kill context rows」）。
- 无新依赖；旧库打开时自动迁移到 v12。
