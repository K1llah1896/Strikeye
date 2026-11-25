# Proposal: 选手头像抓取与展示

## Why

调研的 `player-profiles` 一节把"头像/队徽抓取与回退"列为未补项：`cache/radar_cards/portraits` 目录早就在布局里，但没有任何代码往里写，分析页的选手表只能用彩色圆点代替头像，和 CS:DM 的选手卡片观感差一档。

Steam 的公开 XML 资料页（`steamcommunity.com/profiles/<id64>/?xml=1`）不需要 API key 就带 `avatarFull`，所以这件事不需要凭据，也不该再挂在阻塞清单上。

## What Changes

- `internal/avatars`：抓取公开资料页 → 解析 `avatarFull` → 下载图片 → `.part` 原子落盘到 `<DataRoot>/cache/radar_cards/portraits/<steamid>.png|jpg`，默认 30 天复用；私密/不存在资料返回 `ErrNotFound`，非法 SteamID64 返回 `ErrInvalidSteamID`。
- `strikeye-cli avatars`：`--steamid a,b` 或 `--demo-id N`（取该场全部选手），`--refresh`/`--out-dir`，JSON 汇总 fetched/cached/failed。
- 分析页选手表：有缓存头像时渲染 18px 圆形缩略图，否则回到原来的阵营色圆点；解码失败的缓存文件直接跳过。

## Impact

- Affected specs: `player-profiles`（新增 requirement）
- Affected code: `internal/avatars`、`cmd/strikeye-cli/avatars.go`、`internal/ui/views/analysis`、`main.go`
