# 开镜收窄视野锥

## 为什么

可见性与击杀光学一直按固定 53° 半角判定，但狙击手是眯着眼开镜杀人的：AWP 一级开镜的投影视野远窄于奔跑时。按 53° 判定会把"开镜后其实只看得见很小一片"的击杀算成"他一直看着你"，TTD 与 FOV/背身比例都会偏乐观。

demoinfocs 提供 `Equipment.ZoomLevel()`（`ZoomNone/ZoomHalf/ZoomFull`），回放里也能逐帧记录，所以不需要新数据源。

## 变什么

1. 回放格式升到 **v3**，`ReplayPlayerPose` 增加 `Zoom uint8`；v2 文件读回 `Zoom=0`（旧缓存不失效）。
2. 解析采样把开镜等级写进回放（`aw.ZoomLevel()`）。
3. 可见性判定按开镜等级收窄半角：未开镜 53°、一级 20°、二级 7.5°；任一帧不满足即中断游程。
4. 击杀光学（`internal/killctx`）的 `in_fov` 用攻击者开镜半角、`victim_sees_killer` 用受害者开镜半角，并输出 `attacker_zoom` / `victim_zoom`。
5. `docs/metrics.md` 与调研报告同步口径。

## 影响

- capability：`visibility-metrics`（MODIFIED「Optics-aware visibility」+ ADDED「Scope-aware cone」）。
- 回放缓存 v2 → v3 需要重解析才能拿到 Zoom；不重解析时行为与之前一致（Zoom=0）。
- 指标数值会变：真实 dust2 上 KSCERATO TTD 455.7ms→483.2ms（样本 37→27）、NiKo 1263.7ms→940.9ms（23）。
