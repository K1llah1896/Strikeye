# 指标口径

本文记录 Strikeye 每一个分析指标的算法，对应实现位于 `internal/parser/metrics.go`、`internal/mapcontrol`、`internal/killctx` 与 `internal/zones`。目标只有一个：任何人在 Excel、SQL 或别的工具里都能用原始字段复算出同样的数字。

## 回合分母

所有 per-round 指标都用**该选手实际参与的回合数**作分母（`rounds_played`），不是全场比赛回合数：

- 中途上场的教练/替补只统计上场回合；
- 分侧统计（`match_player_sides`）时，分母是该侧的回合数。

## 基础计数

| 指标 | 公式 | 说明 |
| --- | --- | --- |
| K/D | `kills / deaths` | 分母为 0 时记 0 |
| KPR | `kills / rounds` | 每回合击杀 |
| DPR | `deaths / rounds` | 每回合死亡 |
| APR | `assists / rounds` | 每回合助攻 |
| ADR | `Σ damage / rounds` | 伤害取 `PlayerHurt` 的 `HealthDamageTaken`，含友伤以外的全部来源 |
| Utility ADR | `Σ he_damage + molotov_damage + inferno_damage / rounds` | 只统计手雷/燃烧弹 |
| HS% | `headshot_kills / kills × 100` | 爆头率 |
| 生存率 | `survived_rounds / rounds × 100` | 回合结束时仍存活 |

## KAST

`KAST% = (K + A + S + T) 回合数 / rounds × 100`

- **K**：该回合有击杀；
- **A**：该回合有助攻；
- **S**：存活到回合结束；
- **T**：死亡但被队友在 5 秒内换掉（`traded`）。

一个回合只计一次，重叠时取并集。

## Rating（Strikeye Rating 2.0 风格）

沿用 HLTV 2.0 的回归系数：

```
impact = 2.13 × KPR + 0.42 × APR − 0.41
rating = 0.0073 × KAST + 0.3591 × KPR − 0.5329 × DPR
       + 0.2372 × impact + 0.0032 × ADR + 0.1587
```

保留两位小数，平均 1.00 为联盟平均线。

`impact` 单独入库，用于解释"rating 高是因为人头多还是因为助攻多"。

## 经济调整 Rating 与 Round Swing

**Round Swing**：每个击杀对当局胜率的边际影响。用 CT 存活人数 × T 存活人数的胜率表（`ctWinProbTable`）查得击杀前后 CT 胜率之差，T 方取反；安放炸弹后按爆炸/拆除模型修正。回合内逐击杀累加，按回合数取平均。

**经济调整**：用"对手装备价值 / 本队装备价值"作权重，以弱胜强时权重高、打弱队时权重低：

```
ratio    = clamp(opponent_equip / own_equip, 0.45, 1.65)
ecoKill  = ratio          # 击杀、助攻、伤害都乘这个权重
ecoDeath = 1 / ratio      # 死亡反向：经济优势时死亡更亏
ecoAdjRating = 0.0073 × KAST + 0.3591 × ecoKPR − 0.5329 × ecoDPR
             + 0.2372 × ecoImpact + 0.0032 × ecoAdjADR + 0.1587
             + 0.015 × roundSwing
```

经济值取自 `match_rounds.team_a_equip_value / team_b_equip_value`，任一侧 ≤ 0 时权重退回 1.0（例如半场首回合数据缺失）。与 HLTV 3.0 的"经济调整 + Round Swing"思路一致，但系数是本地回归，**不能**与 HLTV 官方数值直接比较。

## 对枪与突破口

| 指标 | 定义 |
| --- | --- |
| First Kill / First Death | 回合内第一个击杀/第一个阵亡 |
| Opening Win Rate | 拿到首杀的回合中最终获胜的比例 |
| Trade Kill | 队友在 5 秒内击杀刚刚杀死自己的对手 |
| Clutch | 1vN 残局；`clutch_attempts` 记进入残局次数，`clutch_wins` 记赢下次数 |
| Multi-kill | 单回合 2/3/4/5 杀的回合数，`multi_kill_rounds` 为 ≥2 杀的回合数 |

## 时间类指标

**TTK（Time To Kill）**，单位毫秒，取自 `match_players.ttk_avg_ms`：

- 以"第一次命中该受害者的攻击者"为交战起点；
- 3 秒窗口（`duelWindowTicks = 3 × tickRate`）内同一攻击者的后续命中延续交战；
- 攻击者更换、窗口超时、受害者死亡都会终止当前交战；
- 只统计最终造成击杀的交战，`ttk = (首杀 tick − 首次命中 tick) / tickRate × 1000`。

**TTD（Time To Damage）**，单位毫秒，取自 `match_players.ttd_avg_ms`：

- 解析时以 32Hz 采样存活选手眼位（位置 + 64）、眼角 yaw 与**开镜等级**，保留 5 秒环形缓冲；
- 每次"首次伤害"向前回溯，找连续可见游程（BVH 遮挡判定，端点容差 1e-4）；
- 一帧算"可见"需要同时满足：无遮挡、攻击者未被闪（`FlashDurationTimeRemaining > 0.1s`）、受害者在攻击者水平视锥内；
- 视锥半角随开镜收窄：未开镜 53°、一级开镜 20°、二级开镜 7.5°（回放格式 v3 起记录 `Zoom`，v2 缓存读回为 0）；
- `ttd = 首个可见采样点 → 伤害 tick` 的时间差；
- 没有地图几何数据时该值记 0，解析照常完成（`ttd_samples = 0` 表示"不可计算"而不是"立刻命中"）。

两者都是对枪的**代理指标**：回放只有 32Hz 采样和命中事件，没有逐发子弹的 server tick，因此毫秒级精度不可外推到单场比赛之外。

## 空间类指标

| 指标 | 定义 |
| --- | --- |
| 区域（callout） | `callouts.csv` 的世界坐标盒；优先最小面积命中，否则取 512 单位内最近盒心 |
| 默认站位 | 回合 `freeze_end_tick + 20 秒` 的就近帧，只统计存活选手 |
| 击杀区位 | 阵亡者坐标归属的 callout |
| 地图控制 | 每队多源 Dijkstra（截断 1100 单位），距离差 < 200 为争夺；烟雾遮蔽与火焰封路区域不计归属 |
| 控制净值 | `(CT 区域数 − T 区域数) / 已分类区域数`，范围 [-1, 1] |
| 交火距离 | 击杀者眼位（+64）到受害者胸口（+45）的欧氏距离 |
| 视线 | BVH 判定该连线是否被几何遮挡 |
| 击杀光学 | 攻击者/受害者 yaw 与连线的夹角；`in_fov` 用开镜后的视锥（53°/20°/7.5°），`victim_sees_killer` 用受害者自身视锥 |
| 穿烟 | 击杀连线到烟心（+32）的最短距离 < 144 |
| 武器归属 | 同槽位 ±64 tick 内最近一发 `weapon_fire`（`parser.NearestShotWeapon`），击杀信息条与导出口径一致；刀/雷击杀为空 |

## 击杀上下文落库

`killctx --store` 把上面的空间类判定写进 `kill_context` 表（迁移 v12），供 SQL 与导出使用：

| 列组 | 列 |
| --- | --- |
| 定位 | `round_number`、`tick`、`time_sec`（相对 `freeze_end_tick`） |
| 双方 | `killer_slot`、`victim_slot`、`killer_name`、`victim_name`、`weapon`、`headshot` |
| 几何 | `distance`、`visible`、`geometry_checked`、`smoke_blocked`、`killer_x/y/z`、`victim_x/y/z` |
| 光学 | `optics_checked`、`aim_off_deg`、`in_fov`、`victim_sees_killer`、`attacker_blind`、`victim_blind`、`attacker_zoom`、`victim_zoom` |

主键 `(demo_id, round_number, tick, victim_slot)`，同一 tick 的多人死亡不会互相覆盖。`Totals` 在 SQL 里聚合击杀数、平均距离、背身率（`geometry_checked=1 AND visible=0`）与穿烟率。导出侧对应 `match_<id>_kills.csv`、XLSX `kills` 表与 JSON `kills`。

## 片段与高亮

高亮评分与片段边界属于渲染层（`internal/parser` 的 highlight 评分 + `internal/composer`），阈值写在 OpenSpec `competitive-highlights` 与 `render-scheduler-and-safety` 里，这里不重复。

## 版本

指标口径一旦变更必须：

1. 在 OpenSpec 里写 change 并说明影响（旧值与新值不可直接比较）；
2. 同步更新本文；
3. 在导出文件里保留 `schema_version`，让下游知道用的是哪一版口径。
