## 1. 生成

- [x] 1.1 steam:// 启动 URL，路径百分号编码、`+` 额外转义
- [x] 1.2 正数 tick 才生成 `demo_goto` cfg
- [x] 1.3 输出 CS2 友好文件名

## 2. CLI

- [x] 2.1 `strikeye-cli play --demo-id [--tick]` 输出 JSON
- [x] 2.2 缺少 demo-id 时报错

## 3. 验证

- [x] 3.1 单测：转义、单一 `+`、tick 门控、文件名
- [x] 3.2 真实 demo 调用输出符合预期
- [x] 3.3 `go test ./...` 全绿；`openspec validate open-demo-in-cs2 --strict` 通过
