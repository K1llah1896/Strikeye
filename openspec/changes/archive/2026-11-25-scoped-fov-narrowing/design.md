# 设计

## 决策

**为什么不用 FOV 精确投影**：demo 的 `CMsgTEFireBullets` 只带 `player_scoped` 布尔值，没有投影矩阵或 fov 字段；`m_bIsScoped` 与 `m_zoomLevel` 实体属性只给等级。因此用等级到半角的标定表：CS2 默认水平视野约 106°（半角 53°），AWP 一级开镜约 40° 水平视野（半角 20°），二级约 15°（半角 7.5°）。

**为什么回放版本而不是重算**：开镜状态是逐帧瞬时量，TTD 回溯需要历史帧的 zoom，事后无法从死亡事件恢复。故写进回放（每姿势 1 字节，整场约 5.7 万帧 ×10 人 = 0.57MB 增量，可接受）。

**向后兼容**：`encodeRoundPayload` 与 `decodeRoundPayloadBytes` 都带 `version`；`version >= 3` 才读写 zoom 字节，v2 文件读回 `Zoom=0`，行为与改动前完全一致。`Bump ReplayFormatVersion=3` 后旧缓存仍可读（`hdr.Version > ReplayFormatVersion` 才拒绝）。

**两处判定共用一张表**：`parser.scopedFOVHalfAngle`（TTD 用，含"未开镜回落 base"语义）与 `killctx.zoomHalfAngle`（击杀光学用）。两个包不互相依赖，重复 3 行常量优于新增公共包。

## 风险

- 20°/7.5° 是标定近似，不是引擎真值；误差方向是"开镜后视野仍略宽"，即 TTD 略偏长，不会低估。
- 旧 .cs2rpl 不重解析则没有 zoom，指标与之前一致但不再是新口径；`corpus` 黄金快照已按 v3 重生成。
