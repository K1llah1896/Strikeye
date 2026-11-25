## 1. 数据获取

- [x] 1.1 manifest 解析与格式校验
- [x] 1.2 size + SHA-256 校验下载，`.part` → rename
- [x] 1.3 命中缓存跳过网络请求
- [x] 1.4 未知地图/校验失败报错且不留半成品

## 2. 解析与查询

- [x] 2.1 `.tri` v1 读取（gzip、magic、三段数组）
- [x] 2.2 节点引用校验，损坏文件报错不 panic
- [x] 2.3 Möller–Trumbore BVH 遮挡查询，1e-4 端点容差

## 3. CLI

- [x] 3.1 `strikeye-cli maps` 列出缓存
- [x] 3.2 `strikeye-cli maps-fetch --map ... | --all`
- [x] 3.3 缺少选择时报错

## 4. 验证

- [x] 4.1 合成四边形单测：加载、校验和、截断、遮挡/可见/端点
- [x] 4.2 httptest 覆盖下载、复用、校验失败、未知地图
- [x] 4.3 真实拉取 de_dust2 / de_ancient 并做视线抽查
- [x] 4.4 `go test ./...` 全绿；`openspec validate map-geometry-cache --strict` 通过
