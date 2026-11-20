# 接入一款鼠标

在 `internal/drivers/` 新建一个 `.go` 文件，声明 `package drivers`，在 `init()` 中调用 `register`。Go 编译同包文件时会自动包含它。真实硬件示例见 [vk_m3_lite.go](../internal/drivers/vk_m3_lite.go)。

## 最小可运行示例

将下面的代码保存为 `internal/drivers/demo.go`，运行 `go run ./cmd/probe`，终端会出现 `Demo Mouse [演示] · 42%`。运行 `go run .` 可以在菜单中选中它。这个示例使用固定值来验证接入流程，验证结束后删除演示文件。

```go
package drivers

import (
    "context"

    "mouse-battery/internal/battery"
    "mouse-battery/internal/transport"
)

func init() { register(discoverDemo) }

func discoverDemo(ctx context.Context, scan *transport.Scan) ([]battery.Device, error) {
    return []battery.Device{{
        ID: "demo/one", Name: "Demo Mouse", Connection: "演示",
        Read: func(context.Context) (int, error) { return 42, nil },
    }}, nil
}
```

## 换成真实硬件

1. **发现和识别。** 对于 HID 鼠标，`scan.HID()` 返回本轮各驱动共享的接口列表。先按硬件 ID 和控制接口筛选。共用接收器时，继续核对回包中的型号标识。未匹配返回空列表。对 I/O 失败返回错误，可以同时返回已确认的其他设备。
2. **返回设备。** `Name` 用于菜单和提示，`Connection` 用于区分连接方式。`ID` 以驱动名为前缀，区分每一只设备。优先使用已验证的设备唯一标识。如果只有接口路径，就用路径区分，同型号的两只鼠标可以分别选择。有线和接收器只有在能证明属于同一只鼠标时才共用 ID，并只返回一个条目。
3. **读取电量。** 将设备路径绑定到 `Read(ctx)`。HID 请求可用 `context.WithTimeout` 设置超时，再通过 `transport.Exchange` 发送输出报告并筛选输入报告。返回设备报告的 0–100 整数百分比。读数失败返回错误，0 是有效读数。如果识别回包已经携带电量，可以像 VK 一样复用本轮样本。其他协议可在驱动中使用自己的传输接口。
4. **验证。** 接上实物，运行 `go run ./cmd/probe` 检查名称和电量，再用 `go run .` 验证切换与断开。最后运行 `go test ./...` 与对应平台的构建命令（或 `task build`）。

## 模块职责

| 目录 | 职责 |
| --- | --- |
| `internal/drivers` | 注册和发现设备。每个型号文件识别实例，并提供读取函数 |
| `internal/transport` | 枚举 HID 接口、打开设备、按时限收发报告 |
| `internal/battery` | 校验电量、管理选中设备和轮询，发布设备列表与当前读数 |
| `internal/ui` | 将同一份状态显示在托盘、菜单栏和悬浮球中 |
