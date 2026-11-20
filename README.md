# Mouse Battery

轻量级跨平台无线鼠标电量显示工具：在 **Windows 通知区域 + 桌面悬浮球**、**macOS 菜单栏** 实时查看鼠标电量，直接通过 HID 读取，无需安装厂商驱动套件。

## 核心特性

- **极简轻量**：Windows 单文件仅 **~2.1 MB**，无需 WebView2 Runtime 或任何第三方 UI 框架，常驻内存极低。
- **各平台原生 UI**：
  - **Windows**：纯 Go（`CGO_ENABLED=0`）直接调用 Win32 API + 系统 GDI+ 绘制高分屏托盘图标、原生菜单和透明分层悬浮球；支持 Per-Monitor DPI v2、多屏边缘吸附、不抢焦点拖拽、Explorer 重启自动恢复及单实例防多开。
  - **macOS**：极简 Cocoa AppKit 桥接，使用原生 `NSStatusItem` 与 `NSMenu`。
- **驱动与 UI 解耦**：底层电量轮询与 HID 通信全平台通用，接入新型号只需新增单个驱动文件。

## 已支持设备

| 型号 | 连接方式 | 驱动源码 |
| --- | --- | --- |
| VK M3 Lite | USB 有线 / 2.4 GHz 接收器 | [vk_m3_lite.go](internal/drivers/vk_m3_lite.go) |

> 欢迎为更多鼠标型号提交驱动 PR，详见 [驱动接入指南](docs/drivers.md)。

## 下载安装

### Windows
前往 [GitHub Releases](https://github.com/bugsos/mouse-battery/releases/latest) 下载对应架构的单文件程序，直接双击运行即可（免安装、零外部依赖）：
- `mouse-battery-windows-amd64.exe`（常规 64 位 Windows）
- `mouse-battery-windows-arm64.exe`（Windows ARM64）

### macOS
当前暂未提供预编译包，可在 Mac 上克隆仓库后运行 `sh scripts/build-macos.sh` 生成本地临时签名的 `bin/Mouse Battery.app`。

## 使用说明

- **切换与刷新**：点击托盘或菜单栏图标可在设备列表中切换鼠标。启动、切换与手动刷新会立即读取，此后每 20 秒自动轮询；手动选中的鼠标断开时显示“未连接”，重新接回同一设备入口后自动恢复。VK 鼠标默认优先使用有线入口。
- **Windows 悬浮球**：支持左键拖拽（不抢占当前前台窗口焦点），靠近屏幕工作区边缘松开后自动吸附；右键可刷新、隐藏或退出，通知区菜单可重新勾选显示悬浮球；重复启动程序时会自动复用已有实例并唤起悬浮球。
- **电量颜色提示**：`< 10%` 红色、`10%–19%` 黄色、`≥ 20%` 绿色，无有效读数时显示 `—`。
- **命令行用法**：
  - `mouse-battery -read-once`：单次读取并在终端输出当前设备与电量（适合脚本集成）。
  - `go run ./cmd/probe`：列出所有已识别鼠标的名称、连接方式及实测电量（适合排查新设备）。

## 从源码构建

需要 Go 1.27.3 或更高版本；macOS 构建额外需要 Xcode Command Line Tools。

| 命令 | 用途 |
| --- | --- |
| `go run .` | 启动桌面应用（macOS 需 `CGO_ENABLED=1`） |
| `go run ./cmd/probe` | 在终端列出每只已识别鼠标及其实测电量 |
| `go test ./...` | 运行单元测试 |
| `CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags="-s -w -H windowsgui" -o bin/mouse-battery.exe .` | 构建 Windows 正式版 |
| `sh scripts/build-macos.sh` | 构建 macOS 应用包 `bin/Mouse Battery.app` |

如已安装 [Task](https://taskfile.dev/)，也可直接使用 `task dev`、`task probe`、`task test`、`task build`。

## 接入新型号与参与贡献

一个鼠标型号对应 `internal/drivers/` 下的一个 Go 文件，名称、识别规则与电量协议均封装在该文件内。完整接入步骤与示例见 [驱动接入指南](docs/drivers.md)。

## 开源协议

本项目基于 [MIT License](LICENSE) 开源。
