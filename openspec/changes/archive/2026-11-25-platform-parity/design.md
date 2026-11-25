## Context

平台相关代码已经分叉，缺的是"能力可见性"。录制路径依赖 HLAE（Windows-only 的 CS2 注入工具），没有跨平台替代；其余功能（解析、回放缓存、2D、几何、导出、SQL）都是纯 Go。

## Goals / Non-Goals

**Goals**

- 一条命令回答"这台机器上哪些能力可用、哪些不可用、为什么"。
- 让编译期平台差异有测试守住（socket 类型、HLAE 标志）。

**Non-Goals**

- 不实现 Linux/macOS 的录制（HLAE 无对应实现，属上游工具限制）。
- 不引入新的跨平台 GUI 依赖。

## Decisions

1. **`doctor` 承载平台报告**：doctor 已经是"环境自检"入口，与其新增子命令，不如把平台能力并进去。
2. **用常量文件而不是运行期字符串比较**：`ipcSocketKind()` 由构建标签决定，Windows 与其它平台的返回值在编译期固定，避免手写 `runtime.GOOS` 判断漂移。
3. **HLAE 能力以布尔值暴露**：不需要用户读文档猜"为什么渲染按钮报错"。

## Risks / Trade-offs

- 报告里的 `hlae_recording` 只表示"平台支持"，不代表 HLAE 已安装；工具链探测仍由 `toolchain` 段负责。
