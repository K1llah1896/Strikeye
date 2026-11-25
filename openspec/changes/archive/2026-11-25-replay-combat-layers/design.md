# Design: 回放交火层

## 格式扩展

CS2RPL01 v2 在现有 InputEdges 块之后追加一个新块，旧读取器读到文件尾即停止，因此 v1 文件仍可读；v2 读取器在块缺失（剩余字节为 0）时返回空切片。

```
uint32 shotCount
  int64 tick, uint8 slot, f32 startXYZ, f32 endXYZ, shortString weapon
uint32 deathCount
  int64 tick, uint8 killerSlot, uint8 victimSlot, uint8 flags(bit0=headshot),
  f32 victimXYZ, f32 killerXYZ
uint32 bombCount
  int64 tick, uint8 kind(1=plant,2=defuse,3=explode), f32 posXYZ
```

`ReplayFormatVersion = 2`；`ReadReplayFileAtomic` 接受 `version <= 2`，更高版本返回 `ErrUnsupportedReplayVer`.

## 采集

- 射击：`events.WeaponFire`，起点取射手眼位近似 `Position()`，终点 = 起点 + 视线方向 × 2000 单位；武器名小写。
- 死亡：复用 `events.Kill`，记录击杀者/受害者 slot 与双方位置，headshot 标志。
- C4：`BombPlanted`/`BombDefused`/`BombExplode` 记录事件位置（爆炸位置沿用本回合安放点）。
- 所有事件按回合归并，槽位来自 `playerSlots`；未知槽位的事件丢弃。

## 绘制

绘制顺序：烟雾 → 燃烧 → 投掷物 → 弹道 → 死亡 → C4 → 选手。
- 弹道：1.5 秒淡出细线（黄白）。
- 死亡：永久灰色叉号；最近 4 秒额外画击杀者→受害者红线与爆头红点。
- C4：安放后显示红色方框；爆炸后 3 秒内画 500 单位半径红色半透明圆。
- 图层开关默认全开，状态仅保存在视图内，不写入配置。

## 兼容性

v1 缓存无交火块，画布图层自然为空；解析器在新版本下重解析即写入 v2。
