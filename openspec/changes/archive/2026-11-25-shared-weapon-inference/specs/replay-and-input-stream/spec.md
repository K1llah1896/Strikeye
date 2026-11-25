# replay-and-input-stream

## ADDED Requirements

### Requirement: Shared kill weapon attribution

武器归属 SHALL 由单一实现提供：

- `internal/parser` SHALL 暴露 `KillWeaponWindowTicks`（64）与 `NearestShotWeapon(round, slot, tick)`，返回同槽位窗口内最近一发、已剥离 `weapon_` 前缀的武器名。
- 2D 击杀信息条与击杀上下文（含导出 `weapon` 列）SHALL 使用该函数，不得各自实现。
- 窗口外或该槽位没有射击记录时 SHALL 返回空字符串。

#### Scenario: Same weapon in canvas and export

- **WHEN** 同一次击杀同时出现在 2D 击杀信息条与 `match_<id>_kills.csv`
- **THEN** 两处武器名一致

#### Scenario: Knife kill

- **WHEN** 击杀由刀完成（无 `weapon_fire`）
- **THEN** 武器为空字符串，UI 隐藏武器图标，导出列为空
