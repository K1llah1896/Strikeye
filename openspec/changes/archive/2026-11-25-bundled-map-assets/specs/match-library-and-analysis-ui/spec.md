# Spec Delta

## Purpose

让界面反映“地图数据随包提供”的事实：设置页展示内置状态，分析页在数据缺失时给源码构建者可执行的指引，而不是让终端用户去点下载。

## ADDED Requirements

### Requirement: Bundled map data status in settings

设置页「地图数据」卡片 SHALL 报告内置数据来源与安装状态：

- 当构建内嵌导航网格与 callout 表时，卡片 SHALL 说明这些数据随程序提供并已写入缓存；
- 当存在 `map_geometry.zip` 边车时，卡片 SHALL 说明几何按图按需解压，并显示边车文件名；
- 当构建未内嵌数据（源码构建）时，卡片 SHALL 明确说明可用联网按钮补齐；
- 卡片 SHALL 提供“从内置包安装”动作，一次写入导航网格、callout 与全部几何。

#### Scenario: Packaged build shows bundled status

- **WHEN** 打开设置页且该构建内置了地图数据
- **THEN** 卡片标题区下方说明“内置离线数据”并列出已缓存导航网格与几何数量

#### Scenario: Install action populates caches

- **WHEN** 点击“从内置包安装”
- **THEN** 状态栏报告导航网格、callout 与碰撞几何的安装数量，计数同步刷新

### Requirement: Analysis tabs self-heal missing map data

分析页的区域控制与球队档案 SHALL 在读取 callout/导航数据前先尝试安装内置数据；仍缺失时提示 SHALL 面向源码构建者（`strikeye-cli assets-pack` 或联网 fetch），而不是要求终端用户下载。

#### Scenario: Zones tab works on a fresh DataRoot

- **WHEN** 在一个空的 DataRoot 上打开区域控制标签
- **THEN** 默认站位与击杀区位直接可读，不需要先运行任何下载命令

#### Scenario: Missing map in the bundle

- **WHEN** 请求的地图不在内置数据集中
- **THEN** 该区块显示“内置包不含该地图”的提示并给出联网补齐命令，其余区块照常渲染
