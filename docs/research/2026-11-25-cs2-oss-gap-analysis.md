# CS2 同类开源项目调研 & Strikeye 能力补齐建议

- 调研日期：2026-11-25
- 调研对象：GitHub 上活跃的 CS2 demo 解析 / 观战 / 分析 / 数据获取类开源项目
- 方法：仓库代码盘点（`/home/killah/Strikeye`，约 2.9 万行 Go）+ 各项目 README/文档/源码结构比对
- 结论口径：星标与版本号均为调研当日快照；"缺口"均指 Strikeye 当前代码中确认不存在的能力（已用 grep 校验）

---

## 1. 结论摘要

1. **Strikeye 的解析与单场分析深度已进入第一梯队**：单遍解析、32Hz 回放缓存、闪光助攻/分侧/残局/Round Swing/经济调整、6 轴雷达、2D 回放、HLAE 精确 tick 录制 + FFmpeg 合成，这套闭环在开源项目里只有 CS:DM 完整具备，且 Strikeye 的原生 UI + 单二进制形态更轻。
2. **最大差距不在"算得准"，而在"数据进得来、画面讲得清、结果出得去"**：
   - 进不来：只识别 Valve/完美/5E 三类来源，无 share code、无 Steam/FACEIT 下载、不解压归档；
   - 讲不清：2D 回放缺弹道、死亡位置、C4 伤害区、画板、上下层雷达，无语音与音轨（本仓库已补齐：弹道/死亡/C4/笔刷/分层雷达/击杀条/语音解码/音轨混音）；
   - 出不去：无 CSV/JSON/XLSX/SQL 导出，无跨比赛聚合，无公开 API。
3. **解释力是下一个分水岭**：cs-demo-analyzer / CS:DM / awpy 都已具备"可见性 + TTD/TTK"或"navmesh + 地图控制"。Strikeye 目前只能说"谁打得准"，说不清"这一枪为什么能打中、这片区域为什么丢"。
4. **信息层级停在"单场"**：所有主流对标都有系列赛/队伍/选手的跨场视图（CS:DM 的玩家标签 + 跨 demo 过滤、cs2-meta-engine 的反针对报告），Strikeye 的 Library 只有单文件维度。
5. **优先级建议**：P0 = 来源覆盖 + 2D 信息密度 + 语音；P1 = TTD/TTK/可见性 + 系列赛/选手档案 + 导出；P2 = GSI / 游戏内覆盖层（navmesh 地图控制、道具 line-up 库、区域控制已落地）。

---

## 2. 同类项目图谱

### A. 全能桌面端（最直接对标）

| 项目 | 形态 | 关键能力 |
|---|---|---|
| **akiver/cs-demo-manager** 2036★ MIT，Win/mac/Linux | Electron/Java+TS 桌面端 + CS 游戏内插件 | 14+ 平台 demo 导入与自动解压、Steam Web API 补昵称/头像/段位、2D viewer（击杀 feed/死亡位置/弹道/C4 伤害区/鸡/画板/上下层雷达/音轨同步/语音提取）、比赛内 timeline 播放 UI（语音指示、按键+速度悬浮、按玩家静音）、HLTV 2.0 指标、玩家标签 + 跨 demo 过滤、VAC 追踪、XLSX/JSON 导出 + SQL 查询、手雷查找、热力图导出、应用内下载 Valve/FACEIT demo、HLAE/FFmpeg 与 TGA/WAV 两条录制链路、内置 PostgreSQL（v3.21, 2026-11-13） |
| **Strikeye** | Go + MyGo 原生 UI 单二进制 | 单遍解析、SQLite 库、32Hz 回放、6 页分析、2D 回放、雷达卡、HLAE tick 精确录制、蒙太奇合成、渲染调度 |

### B. 解析库 / 数据层（能力上限参考）

| 项目 | 形态 | 关键能力 |
|---|---|---|
| **akiver/cs-demo-analyzer** 126★ Go | CLI + Go API + Node 包 | 16 个来源常量（faceit/esl/esea/matchzy/renown/pracc/esplay/esportligaen…）、CSV/JSON/CSDM 导出、**基于 .tri 碰撞几何 + BVH 的可见性与 TTD/TTK**、经济/购买/道具/鸡/人质/教练/聊天全事件、多来源快照回归测试、地图几何生成工具 |
| **LaihoE/demoparser** 746★ | Rust 核心 + Python/JS 绑定 | 查询式 `parseEvent`/`parseTicks`、任意实体属性、生态事实标准 |
| **pnxenopoulos/awpy** 639★ MIT | Python + Polars/Parquet | 17 类数据集、navmesh `find_area/find_path`、`VisibilityChecker`、**地图控制三模型（raycast / vision / reachability）**、plot 全家桶（雷达/帧/热力图/GIF）、win-prob 预测、CLI |

### C. 轻量 2D / Web 观战

- **sparkoo/csgo-2d-demo-viewer** 81★：Go 后端 + Web 2D 回放
- **zenojunior/cs2d** 28★：纯浏览器 2D 回放、热力图、手雷轨迹、可离线

### D. 数据获取

- **claabs/cs-demo-downloader**：登录 Steam 抓 GCPD 或复用 auth code，批量下载 Premier/竞技/搭档 demo
- CS:DM 内置下载器：最近 8 场 Valve MM / FACEIT

### E. 战术分析

- **Twoos123/cs2-meta-engine**：反针对报告、手雷 line-up 聚类（按 impact 排序）、2D 回放、HLTV 2.0；CT 默认站位与 T 默认展开（0:20 时间点，按 callout / A-Mid-B 分区）

### F. 实时 / 行为分析

- GSI HUD：drweissbrot/cs-hud 366★、JohnTimmermann/JTs-Hud 224★、M3MONs/CS2-HUD
- 行为：Driw0x/CS2Guard（ML 作弊检测）、yimingsu01 准星预瞄（YOLO 视频）、explosivebarrel/cs2-demo-analysis-tool（aim & duel 指标）、pr1maly 50+ 指标趋势

### G. 语音

- akiver/csgo-voice-extractor 43★（CLI → WAV）、DandrewsDev/CS2VoiceData 29★（Go 提取）

---

## 3. 能力矩阵

图例：✔ 完整 / ◐ 部分 / ✘ 无

图例：✔ 具备 · ◐ 部分 · ✘ 缺失 · — 不适用。

| 能力 | Strikeye | CS:DM | cs-demo-analyzer | awpy | demoparser | cs2d |
|---|---|---|---|---|---|---|
| 多来源识别（≥8 家） | ✔（18 类） | ✔（14+） | ✔（16） | ◐ | — | — |
| demo 下载 / share code | ✔（GCPD 下载需用户 Cookie；share code 双向） | ✔ | ✘ | ✘ | ✘ | ✘ |
| 归档自动解压 | ✔（zip/gz/bz2/zst） | ✔ | ✘ | ✘ | ✘ | ✘ |
| 语音提取 / 音轨同步 | ✔（原始包 + 发言统计 + 内置 Opus 解码；按说话人分轨 `speaker_<xuid>.wav` + 混合轨、偏移对齐 demo 时间线；回放页语音时间轴（每 0.5s 断句）+ `voice-activity` CLI；合成侧多轨偏移/音量/静音 + 纯音频导出） | ✔ | ✘ | ✘ | ✘ | ✘ |
| 2D：玩家/道具/朝向 | ✔ | ✔ | — | ✔ | — | ✔ |
| 2D：弹道 / 死亡位置 / C4 伤害区 | ✔ | ✔ | — | ◐ | — | ◐ |
| 2D：标记 / 上下层雷达 / 自由笔刷 | ✔（区域标记 + 上下层雷达（每图偏移/透明度）+ 自由笔刷画板 + 击杀信息条） | ✔ | — | ✘ | — | ✘ |
| 可见性 / TTD / TTK | ✔（TTD+TTK） | ✔ | ✔ | ✔ | ✘ | ✘ |
| navmesh / 地图控制 | ✔（可达性） | ✘ | ✘ | ✔ | ✘ | ✘ |
| 经济 / 购买 / 道具事件 | ✔（经济 KPI + 逐件购买时序） | ✔ | ✔ | ✔ | ✔ | ✘ |
| 残局 / 交易 / KAST / ADR | ✔ | ✔ | ✔ | ✔ | — | ✘ |
| CT/T 分侧、Round Swing、经济调整 | ✔ | ◐ | ✘ | ✘ | ✘ | ✘ |
| 跨比赛玩家档案 / 标签 / VAC | ✔（档案 + 标签 + 封禁快照） | ✔ | ✘ | ✘ | ✘ | ✘ |
| 系列赛 / BO3 / 队伍层 | ✔（系列赛分组、队伍档案、侦察报告、交手矩阵，前四项均已进 GUI「队伍视角」标签页） | ◐ | ✘ | ✘ | ✘ | ✘ |
| 导出 CSV/JSON/HTML/XLSX + SQL | ✔ | ✔ | ✔ | ✔（Parquet） | ✔ | ✘ |
| 手雷 line-up 库 / 默认站位 | ✔（聚类 + 后续击杀 + 相对基准胜率的收益归因） | ✔（查找器） | ✘ | ◐（grenades 数据） | — | ✘ |
| 游戏内覆盖层 / 插件 | ✘ | ✔ | ✘ | ✘ | ✘ | ✘ |
| 实时 GSI | ✔（应用内接收端 + 资料库直播条，CLI 形态保留） | ✘ | ✘ | ✘ | ✘ | ✘ |
| 原生 UI（无 WebView） | ✔ | ✘ | —（CLI） | —（库） | —（库） | ✘ |
| HLAE tick 精确录制 | ✔ | ✔ | ✘ | ✘ | ✘ | ✘ |
| 跨平台 | ✔（三平台编译通过；`doctor` 报告平台/IPC/录制可用性——录制仍限 Windows + HLAE，这是 HLAE 本身的约束而非本项目的缺口） | ✔ | ✔ | ✔ | ✔ | ✔ |

---

## 4. 差距逐项分析

### 4.1 Demo 获取与来源覆盖 —— **P0**

**现状（已补齐）**：`internal/watcher/sources.go` 的 `platformRules` 有序表覆盖 18 类来源（valve / perfect_world / 5e / faceit / esl / esea / matchzy / pracc / renown / challengermode / esportal / fastcup / esplay / esportligaen / popflash / ebot / custom / unknown），Library 过滤器与注册表同源（`internal/ui/views/library/platforms.go`）。归档 zip/gz/bz2/zst 自动解压（`internal/watcher/archive.go`），`.info` 旁挂元数据（`internal/watcher/info.go`），官方 demo 可由 `strikeye-cli download` 从 GCPD 拉取（`internal/gcpd`），share code 双向编解码（`internal/sharecode`）。

**对标**：CS:DM 与 cs-demo-analyzer 都覆盖 14–16 家来源，且 CS:DM 支持 zip/gz/bz2/zst 自动解压——玩家真实拿到的 demo 大量是压缩包。

**为什么重要**：来源覆盖决定"打开就有多少比赛可分析"。FACEIT/ESL/MatchZy 是社区复盘与战队训练的主要来源；没有入口，后面的分析全部空转。

**建议**：
1. 把平台识别做成可扩展表（文件名/路径/`cl_` 头/`.info` 联合判定），先补 `faceit`、`esl`、`matchzy`、`pracc`、`renown` 五家；
2. 引入 `.info` 旁挂文件 + Steam Web API（昵称/头像/段位）补全元数据；
3. 归档解压（默认关，解压后仍走现有稳定性 + sha256 去重管线）；
4. share code 解析 + Steam 下载（可先做"粘贴 share code → 下载"单场，参考 cs-demo-downloader 的 auth code 路径，避免账号密码落地）。

成本：识别表 + 元数据 ~1 周；归档 ~0.5 周；下载器 ~2 周（含错误/限流/取消）。

### 4.2 2D 回放信息密度 —— **P0**

**现状（已补齐）**：`internal/ui/components/radar_2d_canvas.go` 现画烟雾（144 units）、燃烧凸包、手雷弧线、玩家（朝向锥/血量/闪光/C4/编号）、弹道（1.2 宽黄金色淡出）、死亡叉号与爆头点、C4 安放/拆除/爆炸范围、击杀信息条（6 秒窗口、官方武器图标）、区域标记、自由笔刷、上下层雷达叠加（每图偏移/透明度）；热力图用 KDE 并支持站位/交火/击杀/死亡多维。

**对标**：CS:DM 的 2D viewer 是事实标准：击杀 feed、死亡位置标记、弹道、C4 爆炸伤害区（CS2 起用地图 shockwave 数据）、鸡、画笔/橡皮、上下层雷达（每地图保存位移与透明度）、音轨同步、快捷键。cs2d 也做了手雷轨迹与热力图。

**为什么重要**：2D 回放是 Strikeye"可观看、可理解"主张的主舞台。当前缺的四项恰好是复盘时最常问的：他从哪打的、死在哪、这颗雷能不能炸到、我要能在图上圈出来讲。

**建议**（数据结构大多已具备，属"画出来"而非"重新解析"）：
- `shots`：解析 `weapon_fire`，用 `shots` + 玩家帧画弹道（分段染色，命中/未命中）；
- 死亡位置：从 kill 事件坐标画 X 标记 + 时间衰减；
- C4 伤害区：优先用地图 shockwave 数据，缺图回退球面近似（与 CS:DM 同一降级策略）；
- 画板：笔/线/箭头/橡皮 + 撤销，按 demo 保存；
- 上下层雷达：每地图持久化 offset/opacity，按 `z` 分层；
- 鸡、掉枪、燃烧瓶落点作为可选图层。

成本：弹道+死亡点+爆炸区 ~1.5 周；画板 ~0.5 周；分层雷达 ~0.5 周。

### 4.3 语音与音轨 —— **P0**

**现状（已补齐）**：`internal/voice` 把 `CSVCMsg_VoiceData` 按说话人/tick 存入 `.cs2voice` 容器（`strikeye-cli voice`），并自带**纯 Go Opus 解码**（`github.com/pion/opus`，无 cgo）：`format=OPUS` 的帧按段号/偏移排序后解成单声道 48kHz WAV（`voice-decode` 默认走内置路径，输出 `mixed.wav` 与每说话人样本数）；Steam 包装格式（`format=0/1`）仍可 `--tool` 委派外部解码器。仓库内两份 fixture 语音包数为 0（公共匹配 demo 不录语音），因此解码链路用 ffmpeg 现场生成的 Ogg/Opus 包做端到端验证：151 个包 → 3.02s WAV，`ffprobe` 核对时长一致。合成侧已有外挂音轨混音与纯音频导出（`audio-tracks`）。

**对标**：CS:DM 支持"Generate audio file"提取语音、外挂音轨（mp3/wav/flac/aac/m4a/ogg）并对齐、按玩家/队伍静音；Valve MM demo 本身无语音，这点要如实提示用户。两个独立项目（csgo-voice-extractor、CS2VoiceData）证明纯解析即可实现，后者就是 Go。

**为什么重要**：POV 高光没有队内语音，叙事力掉一半；而且语音能直接支撑"这回合为什么转点"的分析。

**已落地**：容器 + 发言统计 + 内置 Opus 解码 + 外挂音轨混音/纯音频导出。

**已落地补充**：按说话人分轨 `speaker_<xuid>.wav` + 混合轨、偏移对齐时间线（容器 v2 修正 1.37 秒回绕）。

**已落地补充**：回放页「语音时间轴」（按说话人分行的 `█`/`·` 条形图、0.5 秒断句、回合内裁剪）、`voice-activity` CLI 输出每段相对秒数、`voice-decode --xuid` 单人 solo 导出。

### 4.4 交火解释链：TTD / TTK / 可见性 —— **P0/P1**

**现状（2026-11-25 已补齐 TTD）**：`internal/mapgeom` 读取 `.tri`/BVH 并做 Möller–Trumbore 遮挡查询；解析器按 32Hz 采样眼位、对每次首次伤害反算连续可见时长，落 `match_players.ttd_avg_ms / ttd_samples`（迁移 v5），抽屉与 CSV 可见。真实数据（开镜感知后）：dust2 KSCERATO 483.2ms/27 次、NiKo 940.9ms/23 次，无几何数据根时解析照常、TTD 为 0。TTK 已在 `duel-timing-metrics` 落地；开镜 FOV（`Zoom` 记录于回放 v3）、致盲与视锥判定已并入 TTD 与击杀光学；逐 kill 的 `kill_context` 明细由 `killctx` CLI 提供，并已落库（迁移 v12 `kill_context`、`killctx --store`、导出 `_kills.csv`）。

**对标**：cs-demo-analyzer 用地图 `.tri` 碰撞几何 + BVH 射线做可见性，考虑开镜 FOV、烟雾椭球、闪光，输出 `timeToDamage` / `timeToKill`（无几何/旧图/穿墙/超 5s 记 -1），并配 `cmd/map-geometry` 从 CS2 VPK 生成带 SHA-256 manifest 的几何文件；awpy 的 `VisibilityChecker` 同类；CS:DM 已在产品里落地。

**为什么重要**：这是"从统计到解释"的关键一跳。没有它，2D 回放无法回答"他有没有可能看到"，狙击/架枪/背身指标也无从谈起。

**已落地路径（与建议的差异）**：几何不再从本地 VPK 生成，改为使用公开数据集（见 4.4.1），并以 `map_geometry.zip` 边车随安装包分发、按图按需解压，省掉 VPK 解析与联网步骤。

上述"后续可加"三项均已落地：
1. 可见性修正：开镜 FOV（回放 v3 记录 `Zoom`，视锥 53°→20°→7.5°）、闪光状态（剩余闪光 >0.1s 视为致盲）、烟雾用半径 144 的球体近似（椭球差异见 §9）；
2. 逐 kill 的 `kill_context`：`killctx` 计算 + `--store` 落库（迁移 v12）、导出 `_kills.csv`/XLSX `kills` 表/JSON `kills`，可用 SQL 直接查询；
3. 回归语料：`internal/corpus` 两场 fixture 的黄金快照（回合/事件/高亮/弹道/死亡/购买/选手指标），`corpus` 命令 0 差异即通过 CI 门禁（`STRIKEYE_CORPUS=1`）。

### 4.4.1 地图几何数据来源（本轮新增）

**数据源**：`gitlab.com/akiver/cs2-map-geometry`（tag `v1`），16 张官方图，`manifest.json` 带 `size` + `sha256`。格式为 gzip + `CSDAGEO\0` 头 + 顶点/索引/BVH 节点三段，正是 cs-demo-analyzer 计算 LOS/TTD/TTK 用的同一份数据（de_dust2 6.2MB / 244,628 顶点 / 270k 节点；de_ancient 16.1MB）。

**接入方式**：几何以 `map_geometry.zip`（约 200 MB，含 `manifest.json` + 16 张 `.tri`）随安装包分发在 exe 同目录，首次用到某图时只解压该图并校验 `size`+`sha256`，写入 `<DataRoot>/cache/map_geometry`；`strikeye-cli maps` 列出缓存，`maps-fetch --map … | --all` 仍可用于升级到更新的数据集。`strikeye-cli assets-pack` 负责生成随包文件（发布流水线在编译前调用）。

**许可注意**：该仓库没有 LICENSE 文件，因此 Strikeye 仍不把二进制提交进仓库（`.gitignore` 排除，构建时按钉定校验和下载）；但会随安装包分发（`map_geometry.zip`）并在文档/界面标注来源，后续若能自行从 VPK 生成，应替换该来源。

### 4.4.2 导航网格与 callout 数据来源（本轮补齐）

**navmesh**：`github.com/pnxenopoulos/awpy-data` 按 CS2 `ClientVersion` 发 Release，`navs.zip` 内含 `nav/<map>.nav`（22 张图，7.68MB）。Strikeye 钉定 release `2000950`（CS2 1.42.0.1），按 manifest 的 `bytes` + `sha256` 校验后解压到 `<DataRoot>/cache/map_nav/`，解析器支持 v30–v36（v36 需跳过两个内嵌占位 BinaryKV3），提供点定位（区分上下层）与 Dijkstra 寻路，`navpath` 输出跳数/米数/跑图秒数。

**callout**：`github.com/sink3x/cs2-server-admin-callouts`（MIT）的 `schematic/data/nav_places.csv`，11,577 行、10 张图，列为 `map,name,x1,y1,z1,x2,y2,z2`（世界坐标盒），commit 钉定 + `size`/`sha256` 校验，落 `<DataRoot>/cache/map_data/callouts.csv`。盒宽 25–75 单位（按 nav area 采样），因此归属分两层：盒命中为"精确"，否则回退 512 单位内最近盒心并记为"邻近"。dust2 真实回放实测 184 个采样点：157 精确、27 邻近，平均回退 67 单位。

**为什么不能只靠 nav**：CS2 的 `.nav` 不再含地名（`strings de_dust2.nav` 只有 NavGenParams），地名来自地图实体的 `env_cs_place`，所以两份数据必须分别获取。

**已落地**：导航网格与 callout 表已用 `go:embed` 编入可执行文件（安装包零点击即用），`strikeye-cli navs/navs-fetch/callouts-fetch/area/navpath/zones` + Match Analysis「7. 区域控制」（默认站位分布、击杀区位）。

### 4.5 空间分析：navmesh / 地图控制 / 默认站位 / 手雷库 —— **P1**

**现状（已补齐）**：navmesh 与 callout 数据已落地（4.4.2）；区域级默认站位、击杀区位、**地图控制曲线**（可达性模型 + 每 callout 净值）、**道具 line-up 库**（类型/侧别/投掷点/落点聚类 + 后续击杀 + 相对基准胜率的收益归因）均已实现；`control-effects` 补上烟雾（球半径 144）与火焰（凸包）遮挡修正（dust2 平均遮挡 16.9 区域、峰值 111，净值 +0.20 → +0.156），区域控制页另有逐 callout 正负条形图与分桶 sparkline。

**对标**：awpy `map_control` 三模型（raycast / vision / reachability），按 nav area 输出 CT/T/contested/neutral 与 `net_control`，烟雾挡视线、火焰挡路径，可逐 tick 出图；cs2-meta-engine 做 CT 默认站位与 T 默认展开（0:20）分区统计 + 手雷 line-up 聚类按 impact 排序 + 反针对报告。

**为什么重要**：这是"队伍级"洞察，也是 Strikeye 目前唯一完全没有的分析维度。地图控制曲线能直接把"这回合为什么输"讲成"20 秒丢了 Mid 控制"。

**建议**（分两步，先便宜后昂贵）：
- 区域化（已完成）：callout 分区 + 玩家帧 → 默认站位分布与击杀区位；
- 控制曲线（已完成）：`internal/mapcontrol` 按队伍多源 Dijkstra（半径 1100、争夺阈值 200），输出 T/CT/争夺/中立与净值，按 callout 聚合"决定这张图的区域"；dust2 19 回合 409 采样 0.6 秒算完；
- 手雷库（已完成）：`internal/nadebook` 按 `(类型, 侧别, 出手网格, 落点网格)` 聚类，输出次数/回合/平均出手时间/后续击杀；dust2 442 次投掷聚出 16 条 ≥3 次线位（T 211 / CT 231），ancient 785 次（T 402 / CT 383）；CLI `nades`、区域控制页「道具线位」卡片；
- 道具修正（已完成，`control-effects`）：烟雾（球半径 144、Z 容差 144）只解除归属，火焰（凸包、2D 射线法）同时把区域从 Dijkstra 展开中摘除；`--effects=false` 可复现未遮挡曲线；
- 展开路径（已完成，`replay-route-layer`）：2D 画布「推进路线」图层按 8 tick 步长把每位选手的轨迹连成折线（T 橙 / CT 蓝、当前位置加点），逐帧回放可见推进与轮转；`navpath` 另提供 navmesh 逐段寻路用于离线核对；
- line-up 效果归因（已完成）：线位按"后续击杀 + 相对本队该侧基准胜率"给出收益（`Impact`/`WinRate`），区域控制页展示回合胜率与收益两列。

### 4.6 比赛之上的层：系列赛 / 队伍 / 选手档案 —— **P1**

**现状（已补齐）**：`match-series-grouping` 提供系列赛识别与 Library 分组；`player-profiles` 提供跨场选手档案（显示名/备注/标签）与标签搜索；`vac-risk` 提供封禁快照与徽章；`team-dossier` 提供队伍地图战绩与回合加权阵容；`tendency-report` 提供队伍级侦察报告（分侧战绩、手枪局、经济类型战绩、T 侧下包点、开场击杀区域与耗时，JSON/CSV）。

**对标**：CS:DM 的玩家标签 + 跨 demo 过滤、VAC 追踪、Steam 资料；cs2-meta-engine 的队伍级报告与反针对视图。

**为什么重要**：Strikeye 的叙事是"比赛档案"。档案天然需要"同一个人/同一支队/同一个 BO3"的连续性，否则每场都是孤岛。

**仍需补的可选延伸**（非阻塞，纯增量）：
1. UI 侧的折线图组件（数据已可用 `trends --format csv` 取得，`docs/metrics.md` 记录了口径）；
2. 队徽抓取（选手头像已落地，队徽需要额外的队名→图片映射，且官方图有版权限制，暂缓）。

### 4.7 数据出口与互操作 —— **P1**

**现状（已补齐）**：`internal/export` 提供单场 CSV（players/rounds/events/sides/kills）、JSON（`schema_version`）、XLSX 多表工作簿、HTML 自包含报告，`--format` 支持 `csv|json|xlsx|html|both|all` 与逗号组合；跨场选手聚合 CSV/XLSX；`strikeye-cli sql` 与设置页 SQL 控制台提供只读查询（仅 SELECT/WITH/EXPLAIN、行数上限、结果可存 CSV）。Parquet 与公开 API 记为不做（§6）。

**对标**：cs-demo-analyzer CSV/JSON/CSDM；awpy Parquet；CS:DM XLSX/JSON + SQL 查询。

**为什么重要**：分析师的第一动作是把数据拖进 Excel/Notebook；没有出口，Strikeye 只能当播放器。同时导出也是"可验证性"的来源（指标可被外部复核）。

**建议**：
1. 导出器：单场/多场 → CSV、JSON、XLSX（`internal/export`，复用 storage 查询）；
2. 只读 SQL 控制台（限定 SELECT + 行数上限 + 超时），导出结果为 CSV；
3. 稳定的 JSON schema + 版本号（对齐 4.10 的指标版本化）；
4. 可选：`strikeye-cli export` 支持批处理与脚本化。

成本：~1.5 周。

### 4.8 游戏内覆盖层 / 插件 —— **P2，需取舍**

**对标**：CS:DM 的 in-game playback UI（`csdm_*` convar）把 timeline、语音指示、按键+速度悬浮、按玩家静音搬进游戏内，需安装 CS 插件。

**判断**：这是 CS:DM 的护城河，但与 Strikeye"本地、零侵入、原生 UI"的定位冲突，且需维护插件与 VAC 风险评估。**不建议自建插件**；替代路径是把 4.2/4.3 做到位 + 一键"在 CS2 中打开该 tick"（生成 `playdemo` + tick 跳转 .cfg），拿到 80% 价值、0 插件成本。成本 ~0.5 周。

### 4.9 实时 GSI / 直播 HUD —— **P2（接入已落地，Web HUD 不做）**

**对标**：cs-hud 366★、JTs-Hud 224★ 均为 GSI + WebSocket 的观战 HUD。

**现状（已补齐）**：`gsi-live` change 已落地只读接入——Valve 官方 GSI 载荷解析（数字/字符串容错、`player` 与 `allplayers` 合并）、本地 HTTP 接收端（token 校验、1MB 上限）、`gamestate_integration_strikeye.cfg` 生成、ndjson 日志与 `cache/gsi_latest.json` 原子镜像，配套 `strikeye-cli gsi`；`in-app-gsi-receiver` 让桌面应用自己就能起接收端（设置页一键启动/停止，`App.Close()` 释放端口）；`gsi-live-banner` change 让资料库页顶部直接显示这份状态（地图/回合/比分/阶段/炸弹/存活，15 秒新鲜度、2 秒节流，缺失即隐藏）。

**判断**：数据侧补齐到位，仍**不**做 Web HUD/浏览器覆盖层——那会打破原生 UI 的单二进制优势，且属于另一条产品线。若未来做训练模式，可在现有接收端之上加渲染层，无需重做接入。

### 4.10 工程底座（不显眼但决定可信度）—— **P1**

| 项 | 现状 | 对标做法 | 建议 |
|---|---|---|---|
| 多来源回归语料 | ✔ `internal/corpus` 黄金快照（两场 fixture：回合/事件/高亮/弹道/死亡/购买/逐选手 18 项指标），`corpus` CLI 支持 `--update` | cs-demo-analyzer 每个来源都有快照测试，几何 manifest 带 SHA-256 | 已落地；跨来源快照仍需更多真实 demo |
| 地图几何管线 | 无 | `cmd/map-geometry` 从 VPK 生成 + 校验和 | 4.4 的 `strikeye-cli geo build`，产物带版本与 checksum，缺失时明确降级 |
| 指标定义与版本 | 自研 Rating/eco-adjusted 无文档、无版本号 | HLTV Rating 3.0 公开六子项与算法；CS:DM 标注"估算" | 写 `docs/metrics.md`：公式、口径、版本号（如 `strikeye-rating@2`），UI 悬浮说明；与 HLTV 3.0 并列展示而非混称 |
| 数据库演进 | SQLite + migrations + FTS5（`match_search`，v10）+ `ListPage` 分页 | CS:DM 3.x 转内置 PostgreSQL | ✔ 已补齐：全文检索覆盖队伍/地图/选手/备注/标记备注，2,000 行库分页 7ms、重建 15ms、查询 1.5ms；单机场景 SQLite 足够 |
| 跨平台 | Windows 中心（HLAE/命名管道） | CS:DM Win/mac/Linux | 解析/浏览/分析层先跨平台（去掉 winio 硬依赖），录制层保留 Windows |
| 性能 | 单遍解析已优 | cs-demo-analyzer Go 并发按段解析 | 补大 demo（>1GB、加时赛）基准与内存上限，避免 OOM |

---

## 5. 建议路线（按 OpenSpec change 粒度切分）

**Phase 1（4–6 周）：把"进得来、看得清"补齐**
1. `demo-source-expansion`：平台识别表（+faceit/esl/matchzy/pracc/renown）、`.info` + Steam Web API 元数据、归档解压、修掉 Library `faceit` 能筛不能进的错位。
2. `replay-2d-fidelity`：弹道、死亡位置、C4 伤害区、画板、上下层雷达、可选图层开关。
3. `voice-extraction-and-audio`：语音切分、音轨选择/偏移/静音、合成混音、Valve MM 无语音提示。
4. `sharecode-import`：粘贴 share code → 下载 → 入队解析。

**Phase 2（6–8 周）：把"讲得透"补齐**
5. `map-geometry-and-visibility`：`internal/geo`（.tri+BVH）、`geo build` CLI、kill 上下文（TTD/TTK/穿墙/穿烟）、2D 可视化叠加。
6. `series-and-profiles`：series 表与 Library 分组、player/team 聚合页、标签、VAC。
7. `data-export`：CSV/JSON/XLSX + 只读 SQL 控制台 + schema 版本化。
8. `metrics-docs-and-versioning`：`docs/metrics.md` + UI 口径说明。

**Phase 3（4–6 周）：把"打得赢"补齐**
9. `zone-control`：callout 分区控制/默认站位/展开路径（不依赖 navmesh）。
10. `navmesh-map-control`：reachability 控制曲线 + 失守时刻。
11. `grenade-lineups`：投掷点/落点聚类、成功率、收益排序（已落地为 `utility-lineups`，收益暂用后续击杀代理）。
12. 可选：`open-in-cs2`（playdemo + tick cfg）；跨平台解析层。

---

## 6. 不建议补齐的能力

| 能力 | 理由 |
|---|---|
| 自建 CS 游戏内插件（对标 CS:DM in-game UI） | 维护成本高、需注入游戏、与"零侵入本地工具"定位冲突；用 `open-in-cs2` 替代 |
| Web 版 GSI 直播 HUD / 浏览器覆盖层 | 接入层已落地（`gsi-live` + `gsi-live-banner`）；Web HUD 属另一条产品线，会打破原生单二进制优势 |
| ML 作弊检测（CS2Guard 类） | 数据与标注门槛高、结论风险大，非本产品信任模型 |
| 云端账号 / 在线服务 | 与本地化、隐私优先的定位相悖；导出 + 自托管分享链接已够 |

---

## 7. 参考链接

- akiver/cs-demo-manager — https://github.com/akiver/cs-demo-manager ｜ 文档 https://cs-demo-manager.com/docs/
- akiver/cs-demo-analyzer — https://github.com/akiver/cs-demo-analyzer
- LaihoE/demoparser — https://github.com/LaihoE/demoparser
- pnxenopoulos/awpy — https://github.com/pnxenopoulos/awpy ｜ 地图控制 https://awpy.readthedocs.io/en/latest/map_control.html ｜ 数据集 https://awpy.readthedocs.io/en/latest/datasets.html
- sparkoo/csgo-2d-demo-viewer — https://github.com/sparkoo/csgo-2d-demo-viewer
- zenojunior/cs2d — https://github.com/zenojunior/cs2d
- akiver/csgo-voice-extractor — https://github.com/akiver/csgo-voice-extractor
- DandrewsDev/CS2VoiceData — https://github.com/DandrewsDev/CS2VoiceData
- claabs/cs-demo-downloader — https://github.com/claabs/cs-demo-downloader
- Twoos123/cs2-meta-engine — https://github.com/Twoos123/cs2-meta-engine
- drweissbrot/cs-hud — https://github.com/drweissbrot/cs-hud
- JohnTimmermann/JTs-Hud — https://github.com/JohnTimmermann/JTs-Hud
- HLTV Rating 3.0（2025-08-20 发布，经济调整 + Round Swing，六子项）— https://www.hltv.org/news/42485/introducing-rating-30

---

## 8. 本轮已补齐（2026-11-25）

按 OpenSpec 工作流完成并归档 64 个 change（另加 9 个 `2026-11-18` 的阶段二 change，累计 73 个；spec 库 37 个 capability，`openspec validate --all --strict` 39/39 通过）：

| Change | 对应缺口 |
|---|---|
| `demo-source-expansion` | 16 家来源注册表、归档（zip/gz/bz2/zst）导入、`.info` 元数据、Library 平台过滤与注册表一致 |
| `replay-combat-layers` | 2D 回放新增弹道、死亡标记、C4 安放/拆除/爆炸范围（.cs2rpl v2） |
| `match-data-export` | 单场 CSV/JSON 导出 + 跨场选手聚合导出 |
| `match-series-grouping` | 系列赛识别（2 图以上、比分翻转）、Library 分组、`series` CLI |
| `player-profiles` | 选手档案（显示名/备注/标签）、`profile` CLI、Library 标签胶囊与标签搜索 |
| `duel-timing-metrics` | TTK（首次命中→击杀，3 秒交战窗口）持久化、抽屉展示、导出字段 |
| `default-setup-heatmap` | 开局 20 秒默认站位热区（每回合单帧采样） |
| `team-dossier` | 队伍地图战绩 + 回合加权阵容，`team` CLI |
| `map-geometry-cache` | 公开 `.tri`/BVH 数据集按需下载（校验和）、`maps` / `maps-fetch` CLI、遮挡查询 |
| `visibility-time-to-damage` | 32Hz 眼位采样 + 可见游程，TTD 落库（迁移 v5）、抽屉与 CSV 展示 |
| `demo-sharecode` | 57 符号分享码解码/编码，`sharecode` CLI，6 组 Valve 向量双向验证 |
| `open-demo-in-cs2` | `steam://rungameid/730//+playdemo` 启动链接 + 可选 `demo_goto` cfg，`play` CLI |
| `match-html-report` | 自包含 HTML 比赛报告（内联样式、转义、无外链），`--format html` |
| `map-callouts` | awpy-data 导航网格 + nav_places callout 数据集按需下载与校验、`.nav` 解析/寻路、区域归属、`navs`/`navs-fetch`/`callouts-fetch`/`area`/`navpath`/`zones` CLI、区域控制页 |
| `map-control-curve` | 可达性地图控制模型（截断 Dijkstra + 争夺阈值）、净值曲线、按 callout 控制摘要、`mapcontrol` CLI、控制曲线卡片 |
| `utility-lineups` | 道具 line-up 聚类（类型/侧别/出手点/落点 + 后续击杀）、`nades` CLI、区域控制页「道具线位」卡片 |
| `control-effects` | 地图控制叠加烟雾遮蔽（球半径 144）与火焰封路，净值随遮挡下降，`--effects=false` 可复现旧曲线 |
| `kill-context` | 逐 kill 明细：眼位→胸口交火距离、BVH 视线判定、穿烟判定，`killctx` CLI，击杀区位表新增距离/视线列 |
| `export-xlsx-and-sql` | 零依赖 XLSX 工作簿（比赛包四表 / 选手聚合）、只读 SQL 控制台（语法+关键字+`query_only` 三层防护、行数上限） |
| `kill-optics-and-lineup-impact` | 视线（yaw 夹角/FOV/背身）、致盲击杀；line-up 按队伍侧别基准胜率算收益 |
| `voice-capture` | Steam 语音包按说话人/tick 存入 `.cs2voice` 容器 + 发言统计，`voice` CLI |
| `vac-risk` | 迁移 v6 `player_bans`、Steam 封禁客户端与 24h 缓存、`bans` CLI、资料库封禁徽章、档案附带快照 |
| `demo-download` | 登录态官方 demo 列表解析与 `.bz2` 直下（`download` CLI，产物落 `media/imports` 自动入库） |
| `ttd-optics` | TTD 可见性叠加致盲与 53° 水平 FOV（`visibleRunMSOptics`），与击杀光学同一判据 |
| `voice-tool-decode` | 外部 Steam 语音解码器桥接（`voice-decode` CLI、退出码 10–14 诊断、增量音轨登记） |
| `map-annotations` | 迁移 v7 `map_annotations`、区域标记（◎/◉ 切换、战术标记卡片）、雷达图标记层、`notes` CLI |
| `purchase-events` | 迁移 v8 `match_purchases`、买枪窗口逐件购买（价格表 40 项）、`buys` CLI、回合抽屉购买行 |
| `platform-parity` | 三平台编译验证、`doctor` 新增 `platform` 段（IPC 传输/HLAE 可用性）、Unix socket 回退说明 |
| `tendency-report` | 队伍侦察报告：分图分侧战绩/手枪局/平均装备/每回合道具、经济类型战绩、T 侧下包点分布、开场击杀 callout 与耗时，`tendencies` CLI（JSON/CSV） |
| `head-to-head-and-trends` | 两队历史交手矩阵（分图战绩 + 最近交手，方向归一化）与选手逐场趋势，`h2h` / `trends` CLI |
| `avatar-fetch` | Steam 公开资料页抓取头像（无需 API key）、30 天缓存、`avatars` CLI、分析页选手表 18px 头像与圆点回退 |
| `gsi-live` | Valve 官方 GSI 只读接入：载荷解析（数字/字符串容错、`player`+`allplayers` 合并）、HTTP 接收端、KeyValues 配置生成、状态镜像，`gsi` CLI |
| `gsi-live-banner` | 资料库页 GSI 直播条（地图/回合/比分/阶段/炸弹/存活），15 秒新鲜度与 2 秒节流，缺失即隐藏 |
| `freehand-canvas-annotations` | 2D 雷达自由笔刷：迁移 v9 `map_strokes`、归一化坐标笔迹、指针采样（0.004 步长、释放落库）、区域控制页笔刷卡片、`strokes` CLI |
| `replay-kill-feed` | 2D 回放击杀信息条：`ReplayLayers.KillFeed`（默认开）、6 秒窗口内最多 4 条（击杀者▸死者 + 弹道推断武器 + 爆头点）、`SetSlotNames`/`SetTickRate`、分析页「击杀信息」开关 |
| `lower-level-view-prefs` | 迁移 v11 `map_view_prefs`、下层雷达每图偏移（±512px）与透明度（0.1–1.0）持久化、画布 `SetLowerView` 合成、回放/热力图两处滑杆与「重置」「下层半透明」、`viewprefs` CLI |
| `audio-tracks` | 外挂音轨混音：`composer.AudioTrack`（偏移可负=裁头、音量 0→1.0 上限 4、Mute）、`ParseAudioTrackSpec`（`path@offset[:volume]`）、`MixAudioTracks`（adelay/atrim/amix，视频 copy）、`ExtractAudio`（m4a）、`MontageSpec.AudioTracks`、`mixaudio` CLI |
| `library-fulltext-search` | 迁移 v10 FTS5 `match_search`（队伍/地图/选手/备注/标记备注）、bm25 排序、token 白名单防注入、`ListPage` 分页、`search` CLI、资料库搜索并集、2,000 行压测与 benchmark |
| `voice-opus-decoder` | 内置纯 Go Opus 解码（`pion/opus`）：`format=2` 帧按说话人分组、按段/偏移排序、混合单声道 48kHz WAV；`voice-decode` 默认内置、`--tool` 回退外部解码器；`voice` 容器统一 `demo_<id>.cs2voice` |
| `voice-tracks-and-offsets` | 语音容器 v2（偏移 uint16→uint32，修掉 1.37 秒回绕）、按说话人分轨 `speaker_<xuid>.wav` + `mixed.wav`、偏移补静音对齐时间线、饱和求和 |
| `voice-activity-timeline` | 语音包按 0.5 秒断句聚合成说话段、回放页语音时间轴（`█`/`·` 60 字符、回合内裁剪、按时长排序）、`voice-activity` CLI、`voice-decode --xuid` 单人导出 |
| `scoped-fov-narrowing` | 回放 v3 记录开镜等级，TTD 与击杀光学按 53°/20°/7.5° 收窄视锥；指标变化 KSCERATO TTD 455.7→483.2ms |
| `replay-route-layer` | 2D 画布「推进路线」图层（8 tick 采样、T/CT 着色、末点标记、默认开启）补记进 `replay-and-input-stream`，与 `navpath` 的 navmesh 路径互为对照 |
| `regression-corpus` | `internal/corpus` 两场 fixture 黄金快照（回合/事件/高亮/弹道/死亡/购买/选手指标，浮点容差 0.11）+ `corpus` 命令与 `STRIKEYE_CORPUS=1` 门禁 |
| `trend-sparkline` | 选手抽屉的 Rating/ADR 跨场 sparkline 与最近三场摘要（`profile.Trend` 逐场升序） |
| `export-format-all` | `--format all` 一次输出 csv/json/xlsx/html 全部文件（含 `_kills.csv`），并把 `_kills.csv` 与 `html` 写进 `match-data-export` 的 Per-match export 要求 |
| `in-app-gsi-receiver` | 应用内 GSI 接收端：设置页直接启动 HTTP 监听（`gsi.Store` + Bearer 鉴权），每次更新原子写 `cache/gsi_latest.json` 供资料库横幅读取，`App.Close()` 释放端口；CLI 形态保留 |
| `shared-weapon-inference` | 武器归属统一到 `internal/parser`（`KillWeaponWindowTicks=64`、`NearestShotWeapon`、`WeaponDisplayName`），2D 击杀条与 `killctx`/导出不再各写一份，避免同一次击杀两处显示不同枪 |
| `kill-context-persistence` | 迁移 v12 `kill_context`（28 列：武器/爆头/距离/可见性/穿烟/开镜夹角/在视锥内/背身/致盲/开镜等级/双方坐标，主键含 `victim_slot`）、`killctx --store` 与 `Totals` SQL 聚合、导出 `match_<id>_kills.csv` + XLSX `kills` 表 + JSON `kills`；dust2 实测 137 行、平均距离 760.9、穿烟 10、背身 19 |
| `ui-render-smoke-tests` | `internal/ui/renderutil`（`ui.Render` 无头渲染 + 颜色数判定）+ 五个视图的渲染冒烟测试（资料库含下载/Key 面板与直播横幅、设置含 SQL 结果表、分析八标签页与展开抽屉、合辑音轨编辑器、观战页），把"点开才崩"类问题挡在 CI |
| `search-cjk-substring` | 中文检索兜底：含非 ASCII 时在 FTS 之外跑 `LIKE` 子串匹配并按 `demo_id` 去重（`unicode61` 把整段中文当一个 token，故选「双架」原本 0 命中，实测现已命中备注 `LongA 默认双架`） |
| `avatar-manual-refresh` | 分析页「刷新头像」（忽略 30 天 TTL 重取全队、显示新下载/已缓存）+ 修 `avatars.Client` 零值 nil HTTP 崩溃 |
| `settings-geometry-fetch` | 地图数据卡片补「下载碰撞几何（全部官方图）」与几何计数（`internal/mapgeom`，写 `cache/map_geometry`），修掉"按提示下载完 TTD 仍是 0"的缺口 |
| `settings-gsi-setup` | 设置页 GSI 卡片：监听地址 + token → 生成 `cache/gsi/gamestate_integration_strikeye.cfg`（与 CLI 同路径同渲染），依据 `gsi_latest.json` 的 `received_at` 显示 live/无活动流与地图回合比分存活 |
| `settings-sql-console` | 设置页只读 SQL 控制台：4 个预设 + 自由查询（`Validate` + `query_only` 双保险）、前 20 行表格、结果导出 `media/exports/query_*.csv`，200 行与 15 秒上限 |
| `library-sharecode-import` | 资料库「分享码导入」：`sharecode.Decode` → 五个 match history 标签页定位 → 下载到 `media/imports`，已存在报"该场已导入"；顺手修 GSI 监听竞态测试 |
| `library-ban-refresh` | 资料库「刷新封禁」：全库去重选手查询（`vac.Service.Lookup`）+ Steam Web API Key 存 `cache/steam_api_key.txt`（0600，env 优先）+ 无 Key 只读缓存，徽章即时更新 |
| `montage-audio-tracks` | 合辑导出卡片「额外音轨」编辑区（3 行：路径/偏移/音量/静音），写入 `MontageSpec.AudioTracks` 由既有 `amix` 流程混入，把 POV 语音铺到集锦上 |
| `library-steam-download` | 资料库「从 Steam 下载」面板：粘贴 Cookie（存 `cache/gcpd_cookie.txt`，0600）+ 匹配类型选择 + 后台下载到 `media/imports` + 自动扫描，把 CLI 的 `download` 闭环搬进 GUI |
| `settings-map-data` | 设置页「地图数据」卡片：导航网格张数 / callout 大小 / 检查时间 + 后台下载（复用 `internal/mapdata`），把"去终端跑 navs-fetch"的断点补上 |
| `bundled-map-assets` | 地图数据随包分发：navs.zip（7.7 MB）与 nav_places.csv（780 KB）`go:embed` 进二进制、启动即写入缓存；碰撞几何以 `map_geometry.zip` 边车按图按需解压；`assets-pack` 生成构建输入；热更新一并替换边车；设置页展示内置状态 |
| `player-profile-editor` | 分析页选手抽屉内档案编辑（显示名/备注/标签，走 `profile.Save`，按 SteamID 换草稿），补齐"标签只有 CLI 能写"的入口缺失 |
| `library-export-and-play` | 资料库卡片「导出数据」（CSV/JSON/XLSX/HTML → `media/exports`）与「用 CS2 打开」（`steam://rungameid/730//+playdemo`，`internal/launcher` 三平台映射） |
| `team-tab` | 分析页第 8 个标签页「队伍视角」：队伍档案（分图战绩/阵容）+ 侦察报告（分侧/经济/下包点/开场区位）+ 交手记录，队伍切换与后台加载；`internal/parser` 抽出 `ResolveReplayCachePath`/`LoadReplayCache`/`ReplayRoundList` 供 CLI 与 UI 共用 |
| `bundled-map-assets` | 地图数据全部随包：导航网格 + callout 表 `go:embed` 进 exe（首次运行按 sha256 校验落盘），200 MB 碰撞几何作为 `map_geometry.zip` 边车按图按需解压；启动/GUI 后台自动安装，设置页与 CLI 的下载按钮降级为“从内置包安装”；`strikeye-cli assets-pack` 成为唯一生成入口 |
| `zero-click-refresh` | 最后两类必须联网的数据改为自动补齐：分析页打开比赛时抓取缺失头像（30 天 TTL、命中缓存零请求）、资料库刷新时更新超过 24h 的封禁快照（需 Steam key，`refresh=false` 复用 TTL）；新增 `avatarAutoFetch`/`avatarClient` 注入点让渲染测试保持无网络 |
| `bundled-toolchain` | FFmpeg（119 MB）与 HLAE（9 MB）以 `*-windows-amd64.zip` 边车随包分发，启动前自动解压到 `<DataRoot>/tools/<tool>`；探测用 glob 发现 `tools/` 下的二进制（包内版本优先于 PATH）；打包改用钉定直链（HLAE `v2.193.2`、BtbN `latest`）避开 GitHub API 每小时 60 次的限流；设置页按钮改为离线「从内置包安装」 |
| `release-size-reduction` | 发布包 527 MB → 255 MB 且不减能力：几何边车改存 xz 原始 `CSDAGEO` 负载并只带 11 张竞技图（`--all-geometry` 可全量）、FFmpeg 边车改 `.tar.zst` 只留 ffmpeg/ffprobe、exe `-s -w` + `zip -9`、CJK 字体 GBK 子集 + 覆盖测试（见 §8.1） |
| `ci-gate` | `.github/workflows/ci.yml`：gofmt / vet / test / 三平台交叉编译 / `openspec validate --all --strict`（CLI 钉 1.15.0），语料回归走 `workflow_dispatch`；仓库内 8 个文件统一 gofmt |

## 8.1 发布包体积（2026-11-26，v0.4.0）

0.3.1 的 `strikeye-windows-amd64.zip` 达到 527 MB。按“核心能力一项不减”的前提重做编码与打包后降到 255 MB：

| 组成 | 0.3.1 | 0.4.0 | 做法 |
|---|---|---|---|
| `strikeye.exe` / `agent` / `cli` | 68.0 / 48.7 / 30.4 MB | 62.6 / 43.2 / 31.1 MB | `-ldflags "-s -w"`（符号表与 DWARF 去掉，约 −18%）+ `zip -9` |
| `map_geometry.zip` | 199.7 MB（16 图） | 119.9 MB（11 张竞技图） | 解掉发布方原有的 gzip 包装，改以 xz 帧存原始 `CSDAGEO` 负载（`de_inferno`：40.0→32.6 MB）；休闲图由设置页一键补充 |
| FFmpeg 边车 | 119.5 MB `.zip`（含 ffplay 与文档） | 51.3 MB `.tar.zst`（只留 ffmpeg/ffprobe，固态压缩） | 上游换 gyan.dev essentials（两个 110 MB 二进制，非 master 的 172 MB）；旧 `.zip` 名仍被接受 |
| HLAE 边车 | 9.1 MB | 9.1 MB | 保持上游压缩包 |
| 内嵌 CJK 字体 | 每字重 10.55 MB × 3 个 exe | 每字重 7.7 MB | GBK 子集 + 源码非 ASCII 字符集，`scripts/subset-fonts.sh` 可重建，覆盖测试拦缺字 |

校验：全新 DataRoot 跑内置安装 → `22 nav meshes, callouts=1`，解析真实 de_dust2 demo 后按需解压 `de_dust2.tri` 16.6 MB，`killctx` 平均交火 760.9 单位 / 遮挡 20.4% / 穿烟 7.3%，TTD 569.7 ms（161 样本）、TTK 509.1 ms（63 样本），与压缩前完全一致。开放数据集的 xz 编码代价是首次用到某图时约 1–5 秒解码，之后常驻缓存。

## 9. 仍未补齐及原因

| 能力 | 阻塞原因 |
|---|---|
| Steam 包装语音格式（`format=0/1`）的内置解码 | 分轨、偏移对齐、时间轴指示都已落地；剩下的只有 CS:GO 时代 `STEAM`/`ENGINE` 包装包，需要 SILK/Steam 私有编解码，目前走 `--tool` 外部解码器（`csgo-voice-extractor`），属老 demo 兼容而非主轴能力。 |
| 语音频道区分（队伍语音 / 全部语音） | `CMsgVoiceAudio` 只给 `section`/`sequence`，没有频道字段；时间轴只能表示"谁在说话"。要区分频道得从 `svc_VoiceInit`/语音路由推断，收益低、误判风险高。 |
| 开镜 FOV 的引擎精确值 | 已用回放 v3 记录的 `Zoom`（0/1/2）把视锥收窄为 53°→20°→7.5°，TTD 与击杀光学都吃这套判定；残余差异在于 demo 不暴露投影 FOV 本身，20°/7.5° 是标定近似。 |
| Steam 登录态相关（需凭据） | `download` 已实现 Cookie 解析与 GCPD 分页下载；实际拉取需要用户自己的 Steam Cookie，属凭据而非能力缺口。 |
| 选手头像的离线预置 | 头像按 SteamID 抓取（`steamcommunity.com/profiles/<id>?xml=1`），目标集合随用户导入的 demo 变化、数量无上界，无法预先打包；现在打开比赛即自动抓取并缓存 30 天，用户不再需要点按钮。 |
| 内置地图/工具数据无法进 git | 导航网格 8.5 MB、callout 780 KB、碰撞几何 120 MB、FFmpeg 51 MB、HLAE 9 MB 都是构建输入而非源码，仓库只提交 `assets-pack` 与钉定校验和；发布 zip 内全部具备（见 §8.1），源码构建则按需生成。 |
| Web 版 GSI 直播 HUD（P2 残项） | GSI 只读接入与资料库直播条已落地；浏览器/覆盖层渲染与“零侵入本地工具”定位冲突，仍不做。 |
