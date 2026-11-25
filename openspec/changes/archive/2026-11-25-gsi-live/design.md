## Context

GSI 载荷的字段类型不稳定：`phase_ends_in` 是字符串，`health`/`money` 有时是数字有时是字符串，`weapons` 是以 `weapon_0` 为键的对象。用一个严格的 struct 解析会在真机上随机 400。

## Goals / Non-Goals

**Goals**

- 任意一版 GSI 载荷都能解析，坏字段降级为 0 而不是整包失败。
- 配置生成正确（游戏侧要的是 Valve KeyValues，不是 JSON）。
- 状态可被 UI/脚本消费（文件镜像 + ndjson 流）。

**Non-Goals**

- 不做实时录制/自动切片（属于录制管线，Windows/HLAE 侧）。
- 不做鉴权体系，只有一个可选共享 token。

## Decisions

1. **`flexInt` / `flexFloat`**：带引号/不带引号都能解析；非法值返回 0，避免"一个字段毁了整场比赛"。
2. **`allplayers` 与 `player` 合并**：`allplayers` 是全体视角，`player` 是被观察者；合并时用 steamid 去重，`player` 优先（信息更全，含弹药）。
3. **配置用 KeyValues 字符串模板**：Valve 的 cfg 不是 JSON；测试按结构断言而不是 `json.Unmarshal`。
4. **状态镜像到 `cache/gsi_latest.json`（`.part` → rename）**：UI 轮询文件即可，不需要 UI 与 CLI 之间再开一个 IPC 通道。
5. **`--once` 带超时**：便于冒烟测试与 CI（没有载荷就报错退出，不挂死）。

## Risks / Trade-offs

- 文件轮询有最多几百毫秒延迟：对"看比分/存活人数"足够，对逐帧不适用。
- 仅本地回环默认监听；改成 `0.0.0.0` 需用户显式指定。
