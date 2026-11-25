# Tasks

## 1. Fetch pipeline

- [x] 1.1 `internal/avatars`: `CacheDir`/`Path`/`ValidSteamID64`、`Client{HTTP, ProfileURL}`、`Fetch(ctx, steamID, dir, maxAge)`
- [x] 1.2 资料页 `avatarFull` 解析、`.part` 原子写、30 天 TTL、`ErrNotFound`/`ErrInvalidSteamID`、体积上限

## 2. CLI

- [x] 2.1 `cmd/strikeye-cli/avatars.go`: `runAvatars`（`--steamid`/`--demo-id`/`--out-dir`/`--refresh`），JSON 汇总
- [x] 2.2 `main.go` 子命令与 usage

## 3. UI

- [x] 3.1 `analysis_avatars.go`: `loadAvatars`（解码缓存、跳过坏文件）、`View.avatars`、`avatarFor`
- [x] 3.2 选手表渲染 18px 头像，缺失回退阵营色圆点

## 4. Verification

- [x] 4.1 `internal/avatars/avatars_test.go`: 下载/复用/过期重取、非法 ID、私密资料、JPEG 扩展名
- [x] 4.2 `internal/ui/views/analysis/analysis_avatars_test.go`: 只解码名册内缓存、坏文件跳过、`avatarFor`
- [x] 4.3 真实数据：`avatars --steamid 76561198058500492` 拉到 184×184 JPEG（18,824 字节）
