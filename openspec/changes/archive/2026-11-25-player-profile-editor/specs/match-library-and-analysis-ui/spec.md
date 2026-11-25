# match-library-and-analysis-ui

## ADDED Requirements

### Requirement: Player profile editor

分析页选手抽屉 SHALL 提供档案编辑：

- 抽屉 SHALL 显示「档案」一行，包含显示名、备注、标签（逗号分隔）三个输入与「保存档案」按钮。
- 展开选手时 SHALL 从 `player_profiles` 读取一次草稿；切换到另一位选手时 SHALL 换成该选手的草稿，不得残留上一位的文本。
- 保存 SHALL 走 `profile.Save`：空显示名/备注保留旧值，标签按中英文逗号拆分并归一化。
- 保存成功 SHALL 显示「已保存」，失败 SHALL 显示错误文本。
- 存储不可用时 SHALL 显示只读提示，不得 panic。

#### Scenario: Edit and persist

- **WHEN** 用户输入显示名 `yuurih`、备注 `support`、标签 `support, anchor` 并点击保存
- **THEN** `player_profiles` 中该 SteamID 的三个字段被写入，标签为两项

#### Scenario: Switch players

- **WHEN** 用户展开选手 A 编辑后不保存，展开选手 B
- **THEN** 输入框显示 B 的档案（或空），不显示 A 的文本
