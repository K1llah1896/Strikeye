# Tasks

- [x] 1.1 迁移 v11 `map_view_prefs` + `MapViewPrefsRepo`（Get/Upsert/List、钳制）
- [x] 1.2 画布 `SetLowerView` + `blitLowerUnderlayLocked`（偏移/alpha 合成、默认走快路径）
- [x] 1.3 分析页 `analysis_lower_view.go`（载入/应用/保存 + 滑杆与快捷按钮），回放与热力图两处接线
- [x] 1.4 CLI `viewprefs`（list / `--set`）
- [x] 2.1 测试：仓库默认值与钳制与重开持久化、画布偏移/淡化/重置逐像素、视图载入保存往返、CLI set/list/错误分支
- [x] 2.2 真实数据：`viewprefs --data-root dist --set --map de_dust2 --x 20 --y -10 --opacity 0.55` 后列表回读一致
- [x] 2.3 `go build ./...`、`go vet`、`go test ./...` 全绿；`openspec validate --all --strict` 通过
