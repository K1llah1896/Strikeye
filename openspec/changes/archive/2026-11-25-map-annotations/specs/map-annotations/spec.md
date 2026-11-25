# map-annotations

## ADDED Requirements

### Requirement: Annotation storage

系统 SHALL 以 `(demo_id, round_number, callout_id)` 为唯一键持久化战术标记：

- 每条标记 SHALL 带 `note`、`color`、`created_at`。
- 颜色 SHALL 归一化为 `amber`、`red`、`green`、`blue` 之一，未知值与空值 SHALL 回退为 `amber`。
- `callout_id` 为空 SHALL 返回错误且不写库。
- 同一键重复写入 SHALL 更新 `note` 与 `color`，SHALL NOT 产生重复行。
- 列表 SHALL 按创建顺序返回，并支持按 `demo_id` 过滤。

#### Scenario: Upsert by key

- **WHEN** 先以 (7,3,"LongA") 写入"默认双架"，再以同一键写入"改走 A 小"且颜色 red
- **THEN** 只有一行，note 为"改走 A 小"、color 为 red、id 不变

#### Scenario: Unknown color

- **WHEN** 写入 color="nonsense"
- **THEN** 存储为 amber

#### Scenario: Missing callout

- **WHEN** callout_id 为空白
- **THEN** 返回错误且不新增行

#### Scenario: Delete twice

- **WHEN** 连续两次按同一键删除
- **THEN** 第一次返回 true，第二次返回 false

### Requirement: Annotation pins on the 2D radar

雷达画布 SHALL 能绘制与回放帧无关的标记：

- 每个标记 SHALL 由世界坐标 (X,Y)、地名与四种颜色之一构成。
- 标记 SHALL 绘制在回放帧（含选手、弹道、死亡、C4）之上。
- 无回放帧（空回合或未加载）时标记 SHALL 仍被绘制。
- `SetPins(nil)` SHALL 清除全部标记。
- 世界坐标投影出画布范围的标记 SHALL 被跳过。

#### Scenario: Pin visible without a frame

- **WHEN** 传入一条 pin 且渲染空回合
- **THEN** 画布上出现标记像素

#### Scenario: Clearing pins

- **WHEN** 再次以 nil 调用 SetPins 并重绘
- **THEN** 画布恢复为无标记状态

### Requirement: Zone board annotation workflow

区域控制页 SHALL 允许在工作流内增删标记：

- 默认站位表每行 SHALL 显示标记状态（◎ 未标记 / ◉ 已标记）且可点击切换整场（round 0）标记。
- 页面 SHALL 展示「战术标记」列表，含地名、备注与"移除"操作。
- 标记变化后 SHALL 立即反映到雷达画布，SHALL NOT 阻塞界面。
- 标注位置 SHALL 取该 callout 的世界坐标中心。

#### Scenario: Toggle a callout

- **WHEN** 点击未标记区域的 ◎
- **THEN** 该区域被写入标记，按钮变为 ◉，雷达画布出现对应圆圈

#### Scenario: Remove from list

- **WHEN** 在战术标记列表点击"移除"
- **THEN** 该标记从库中删除且画布上的圆圈消失

#### Scenario: Unknown callout in dataset

- **WHEN** 标记引用的 callout 不在当前地图数据集内
- **THEN** 该标记被跳过，不影响其它标记绘制

### Requirement: Annotation CLI

`strikeye-cli notes` SHALL 支持 `--demo-id`、`--round`、`--callout`、`--note`、`--color`、`--delete <id>`、`--clear`：

- 指定 `--callout` SHALL 写入标记；指定 `--delete` SHALL 按 id 删除，id 不存在 SHALL 报错；`--clear` SHALL 清空该 demo 的全部标记。
- 每次调用 SHALL 回显 `demo_id`、`count` 与标记列表；空列表 SHALL 序列化为 `[]`。

#### Scenario: Add then list

- **WHEN** 以 `--round 3 --callout LongA --note 默认双架 --color red` 调用
- **THEN** 输出 count=1 且标注含 round_number=3、note 与 color

#### Scenario: Clear

- **WHEN** 以 `--clear` 调用
- **THEN** 输出 count=0 且 annotations 为 []

#### Scenario: Missing id

- **WHEN** `--delete 999` 指向不存在的标记
- **THEN** 返回错误"annotation 999 not found"
