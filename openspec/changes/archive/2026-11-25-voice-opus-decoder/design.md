# 设计

## 决策

**数据路径**：`CSVCMsg_VoiceData.audio` 是 `CMsgVoiceAudio`，其中 `format` 有三种取值（`VOICEDATA_FORMAT_STEAM=0`、`ENGINE=1`、`OPUS=2`）、`voice_data` 是负载、`sample_rate` 与 `uncompressed_sample_offset` 描述时间线。CS2 默认发送 `format=2`，`voice_data` 就是裸 Opus 包，因此内置解码只需处理 Opus。

**为什么不用 Steam 容器解析**：Steam 包装格式是 `57 5c 35 00` 头 + flags + 类型化子包（0=silence、6=Opus PLC、11=sample rate、3=uncompressed、4=SILK）。解析它需要处理 PLC 与 SILK 分支，收益只在"极老的 demo"，而这些 demo 用外部工具更合适。内置路径因此对 `format != 2` 的帧**跳过并计数**（`skipped_frames`），不整体失败。

**混合策略**：所有说话人混成一条单声道 48k 轨。分析场景需要的是"这一回合谁在喊什么"，多轨需要路由 UI 与对齐逻辑，属于后续需求；单轨已能支撑回放/合成时叠加。

**帧排序**：按 `(Section, Offset, Tick)` 升序。`section_number`/`uncompressed_sample_offset` 是 Steam 给出的解码时间线，比 tick 更精确；tick 只用于同一偏移内的稳定性。

**解码器**：`github.com/pion/opus`（纯 Go，RFC 6716）。每说话人一个 `Decoder`，`Init(48000, 1)`，缓冲区 5760 样本（Opus 单帧最大 120ms）。单帧解码失败计入 `decode_errors` 并继续，避免一帧损坏毁掉整场。

**WAV 输出**：自写 44 字节 RIFF 头（PCM16 单声道 48k），不引依赖；样本长度写进头，便于下游 ffmpeg 直接读取。

## 兼容与命名

- `voice` 输出 `media/voice/demo_<id>.cs2voice`（与 `media/voice/demo_<id>/` 输出目录同构）。
- `voice-decode` 先用 `demo_<id>.cs2voice`，找不到再按存储里的 `demo.Filename + ".cs2voice"` 兼容旧捕获。
- 内置解码不需要数据库行，因此 demo 未入库也能解（`runVoiceDecode` 在最前面分派）。

## 测试策略

仓库内的两份 fixture 语音包数量为 0，无法用真实 demo 验证解码。测试用 ffmpeg 现场生成 Ogg/Opus（`sine` 音源，libopus 编码），`ParseXOggPackets` 取出原始 Opus 包，再按 `format=2` 封装成帧：

- 包级：混合多说话人、跳过非 Opus 帧、WAV 头与长度自洽、时长接近源。
- CLI 级：`writeSyntheticCapture` 写容器 → `runVoiceDecode` 输出 JSON + WAV；缺容器时报错。
- 真实验证：把合成容器放进 `dist/media/voice` 跑 `voice-decode --demo-id 999`，`ffprobe` 确认 3.02s WAV。

## 风险

- `pion/opus` 对某些编码器参数的兼容性未知；解码失败被计数而非中断，最坏情况是音频部分缺失，不会破坏既有功能。
- 语音只在有语音包的 demo 存在；无语音 fixture 上的验证只能证明"错误路径正确"。
