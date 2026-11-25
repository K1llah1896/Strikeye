## 1. 采样

- [x] 1.1 `setup` 维度：每回合取 freeze-end + 20s 最近回放帧
- [x] 1.2 仅采样该帧存活选手，保持每回合每选手一点
- [x] 1.3 缺少 FreezeEndTick 时以首帧为锚点
- [x] 1.4 TickRate 从演示信息进入 HeatmapFilter，兼容 64/128 tick

## 2. UI

- [x] 2.1 维度下拉新增 setup 并更新说明文案
- [x] 2.2 选手/阵营/回合类型筛选继续生效

## 3. 验证

- [x] 3.1 单测：20 秒帧选择、死亡排除、CT 过滤
- [x] 3.2 `go test ./...` 全绿；`openspec validate default-setup-heatmap --strict` 通过
