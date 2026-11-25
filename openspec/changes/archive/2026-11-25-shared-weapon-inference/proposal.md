# 武器归属逻辑统一

## 为什么

"这次击杀用了什么枪"在两处各写了一遍：2D 击杀信息条（`radar_2d_canvas.go` 的 `killWeapon`）与击杀上下文（`killctx` 的 `weaponFor`）。两者都是"取最近一发"，但窗口常量、前缀剥离和空武器处理各写各的——一旦一边调整（例如把窗口从 64 改成 96 tick），同一个击杀会在画布上和导出里显示不同的枪。

## 变什么

1. 新增 `internal/parser/replay_query.go`：
   - `KillWeaponWindowTicks int64 = 64`（唯一窗口常量）；
   - `NearestShotWeapon(round ReplayRoundData, slot uint8, tick int64) string`（同槽位、窗口内最近一发、剥离 `weapon_` 前缀）；
   - `WeaponDisplayName(raw string) string`。
2. 画布与 `killctx` 都改为调用它，删除各自的私有实现。
3. 测试 `internal/parser/replay_query_test.go`（最近一发优先、前缀剥离、窗口外为空、未知槽位为空、显示名）；画布测试改为直接验证共享函数。

## 影响

- capability：`replay-and-input-stream`（ADDED「Shared kill weapon attribution」）。
- 行为一致化：导出 `weapon` 列与击杀信息条从此同一口径。
