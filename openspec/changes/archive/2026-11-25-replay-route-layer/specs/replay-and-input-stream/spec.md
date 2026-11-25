# replay-and-input-stream

## ADDED Requirements

### Requirement: Route trails

2D 回放 SHALL 提供推进路线图层：

- 每 8 tick（64 tick 下 4Hz）采样一次存活选手位置，并在当前帧额外补一个采样点。
- 折线 SHALL 按队伍着色（T 橙、CT 蓝），透明度低到不遮挡地图细节，当前位置画点。
- 少于两个采样点的选手 SHALL 不绘制。
- 图层 SHALL 可开关且默认开启；关闭后同帧渲染结果不得包含轨迹像素。
- 该图层表示"实际走过的路径"，与 `strikeye-cli navpath` 的 navmesh 最短路径互为对照而非替代。

#### Scenario: Toggle route layer
- **WHEN** 用户取消 2D 回放页的「推进路线」勾选
- **THEN** 同一帧重新渲染后不再出现轨迹，其余图层不受影响

#### Scenario: Stationary player
- **WHEN** 某选手整回合位置不变
- **THEN** 只有末点被画出，不产生零长度折线
