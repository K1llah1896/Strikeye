## 1. 解析

- [x] 1.1 57 符号字母表与 base-57 反向解码，18 字节小端解析
- [x] 1.2 容错 `CSGO-` 前缀、连字符、首尾空白
- [x] 1.3 非法字符/长度/超长载荷返回错误
- [x] 1.4 Encode/Canonical 生成规范分组形式

## 2. CLI

- [x] 2.1 `strikeye-cli sharecode --code` 输出 JSON
- [x] 2.2 空参数报错

## 3. 验证

- [x] 3.1 6 组真实 Valve 分享码编码/解码双向一致
- [x] 3.2 `go test ./...` 全绿；`openspec validate demo-sharecode --strict` 通过
