# 设置页 GSI 配置生成

## 为什么

`gsi-live` 让资料库顶部能显示实时比分，但配置生成只在 `strikeye-cli gsi` 里：不跑一次 CLI，`game/csgo/cfg/` 里就没有配置文件，横幅永远是空的。GUI 用户需要自己能在应用里生成这份配置。

## 变什么

1. 设置页新增「CS2 实时数据（GSI）」卡片：
   - 监听地址与 token 输入，「生成 GSI 配置」写入 `<DataRoot>/cache/gsi/gamestate_integration_strikeye.cfg`（与 CLI 同一路径、同一 `gsi.Config` 渲染）；
   - 「live / 无活动流」徽章依据 `cache/gsi_latest.json` 的 `ReceivedAt`（30 秒窗口）；
   - 有活动流时显示地图、回合、比分、阶段与存活人数。
2. 生成后提示把文件复制到游戏 `cfg` 目录，并说明接收端需要常驻进程（本卡片不代跑守护进程）。

## 影响

- capability：`gsi-live`（ADDED「Configuration card in settings」）。
- 复用 `gsi.Config` / `gsi.State`，无新依赖、无新表。
