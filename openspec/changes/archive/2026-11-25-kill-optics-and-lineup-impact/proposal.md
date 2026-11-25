# Change: Kill Optics and Line-up Impact

## Why

报告 4.9 的收尾项有两块：

1. **可见性的光学修正**：击杀上下文已经有"是否被墙挡住、是否穿烟"，但没有"这一枪是不是打在视野里、对方有没有看着你、有没有被闪"。回放里 32Hz 的 `m_angEyeAngles` 正好能回答。
2. **line-up 收益排序**：道具线位已经有了类型/侧别/落点聚类与"后续击杀"代理，但没法回答"这套烟到底值不值"。

## What Changes

- `killctx` 增加光学层（`Config.FOVHalfAngle=53`、`BlindThreshold=0.1`）：
  - `aim_off_deg`：击杀者视线与"击杀者→受害者"连线的夹角；
  - `in_fov`：夹角是否在水平半视角内；
  - `victim_sees_killer`：受害者视线是否指向击杀者（否则记"背身/偷袭"）；
  - `attacker_blind` / `victim_blind`：击杀 tick 是否处于闪光致盲；
  - 汇总 `optics_used`、`fov_pct`、`ambush_pct`、`blind_kills`；
  - 没有回放帧时 `optics_checked=false`，其余字段照常。
- `nadebook.Lineup` 增加 `wins`、`win_rate`、`impact`、`kills_per_throw`：
  - `impact = 使用该线位的回合胜率 − 该队该侧的基准胜率`，单位为百分点；
  - 聚类内部记录使用过该线位的回合与获胜回合，基准按 `队伍 + 侧别` 统计。
- 区域控制页：击杀区位表的"视线"列追加 `·背身`、`·盲` 后缀；道具线位表新增「回合胜率」「收益」两列（收益 |≥10| 时着色）。

## Impact

- Affected specs: `kill-context`（修改）、`utility-lineups`（修改）
- Affected code: `internal/killctx`、`internal/nadebook`、`internal/ui/views/analysis`
- 兼容性：回放缓存的 `Yaw` 字段语义确认为"眼位 yaw（度）"，无需改格式；旧缓存直接可用
