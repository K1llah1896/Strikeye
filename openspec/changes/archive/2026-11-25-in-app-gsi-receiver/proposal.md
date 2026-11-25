# 应用内 GSI 接收端

## 为什么

GSI 只读接收端此前只有 CLI 形态：想知道"现在是不是在打比赛"，必须先开一个终端跑 `strikeye-cli gsi`，资料库顶部横幅才有数据。桌面应用自己做得到这件事——一个 HTTP 监听而已——却要求用户常驻第二个进程，属于明显的能力错位。

## 变什么

1. `internal/ui/views/settings/gsi_receiver.go`：
   - `gsiReceiver.start(listen, token, dataRoot) (string, error)`：绑定监听（支持 `127.0.0.1:0` 取随机端口），`gsi.Store` + `Store.Handler(token)` 提供鉴权，`Store.OnUpdate` 把每次更新原子写入 `cache/gsi_latest.json`（就是资料库横幅读的那个文件）。
   - `stop()`（`http.Server.Shutdown`，2 秒宽限）、`running()`。
2. 设置页 GSI 卡片新增「启动内置接收端 / 停止接收端」按钮，状态栏显示实际监听地址；`View.Close()` 停止监听，`App.Close()` 调用它，窗口关闭即释放端口。
3. 测试：真实起服务并 POST 一份 state（校验镜像文件与 `Players` 解析）、重复 start 报错、坏地址报错、运行中渲染卡片。

## 影响

- capability：`gsi-live`（ADDED「In-app receiver」）。
- 无新依赖；CLI 形态保留（供无界面/守护场景）。
