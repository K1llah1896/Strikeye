## 1. 聚合

- [x] 1.1 按队伍名（大小写/空白容错）筛选已解析 Demo，支持 A/B 两侧
- [x] 1.2 输出每图比分、胜负、对手、回合数与该图队伍评分
- [x] 1.3 阵容按回合加权 ADR/KAST/Rating，排除对手选手
- [x] 1.4 汇总地图胜负数与回合得失

## 2. CLI

- [x] 2.1 `strikeye-cli team --name <team>` 输出 JSON
- [x] 2.2 空名称与非空查询的边界行为

## 3. 验证

- [x] 3.1 单测：B 侧翻转、大小写、未解析排除、加权 ADR
- [x] 3.2 真实两图验证：FURIA 2-0（16:14、13:6），阵容 5 人
- [x] 3.3 `go test ./...` 全绿；`openspec validate team-dossier --strict` 通过
