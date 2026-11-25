# Spec Delta

## ADDED Requirements

### Requirement: Freehand brush controls in the zones tab

系统 SHALL 在区域控制页提供自由笔刷控件：

- SHALL 提供开/关按钮，开启时按钮文案体现"拖动绘制"。
- SHALL 在存在当前回合笔迹时显示"清除本回合笔迹（N）"按钮。
- SHALL 显示当前回放回合号，并说明回合 0 的笔迹对所有回合可见。
- 关闭笔刷 SHALL 丢弃尚未保存的进行中笔迹。

#### Scenario: Toggle discards pending stroke

- **WHEN** 笔刷处于关闭状态且内存里有未保存点，点击开启
- **THEN** 内存中的待写点被清空
