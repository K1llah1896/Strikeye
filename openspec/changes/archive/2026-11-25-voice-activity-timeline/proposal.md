# 语音时间轴

## 为什么

语音捕获与解码已经落地，但只有"文件"没有"视图"：想知道谁在哪个回合、哪一秒说话，得自己去听 WAV。CS:DM 的回放页有时间轴语音指示，这是"语音能力是否可用"的最后一公里。

## 变什么

1. `internal/voice` 新增 `BuildActivity`：把语音包按说话人聚成"说话段"（0.5 秒内静默合并成一句）。
2. 2D 雷达回放页新增「语音时间轴」卡片：每人一行，用 `█`/`·` 在回合时间轴上画出发言区间，并给出总语音时长与段数。
3. CLI 新增 `voice-activity --demo-id [--round]`，输出每个说话段的相对起止秒数（JSON），便于外部做字幕或剪辑定位。
4. `voice-decode --xuid <id64>` 只导出指定说话人的轨道。

## 影响

- capability：`replay-and-input-stream`（ADDED「Voice activity timeline」）。
- 无语音捕获的 demo 不会报错：卡片显示"本场没有语音捕获"，CLI 输出 `has_voice=false`。
