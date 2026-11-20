//go:build windows

package ui

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"

	"mouse-battery/internal/battery"
)

var (
	modKernel32 = windows.NewLazySystemDLL("kernel32.dll")
	modUser32   = windows.NewLazySystemDLL("user32.dll")

	procGetModuleHandleW = modKernel32.NewProc("GetModuleHandleW")

	procRegisterClassExW              = modUser32.NewProc("RegisterClassExW")
	procCreateWindowExW               = modUser32.NewProc("CreateWindowExW")
	procDestroyWindow                 = modUser32.NewProc("DestroyWindow")
	procFindWindowW                   = modUser32.NewProc("FindWindowW")
	procDefWindowProcW                = modUser32.NewProc("DefWindowProcW")
	procGetMessageW                   = modUser32.NewProc("GetMessageW")
	procTranslateMessage              = modUser32.NewProc("TranslateMessage")
	procDispatchMessageW              = modUser32.NewProc("DispatchMessageW")
	procPostMessageW                  = modUser32.NewProc("PostMessageW")
	procSendMessageW                  = modUser32.NewProc("SendMessageW")
	procPostQuitMessage               = modUser32.NewProc("PostQuitMessage")
	procRegisterWindowMessageW        = modUser32.NewProc("RegisterWindowMessageW")
	procShowWindow                    = modUser32.NewProc("ShowWindow")
	procSetWindowPos                  = modUser32.NewProc("SetWindowPos")
	procGetWindowRect                 = modUser32.NewProc("GetWindowRect")
	procUpdateLayeredWindow           = modUser32.NewProc("UpdateLayeredWindow")
	procSetForegroundWindow           = modUser32.NewProc("SetForegroundWindow")
	procReleaseCapture                = modUser32.NewProc("ReleaseCapture")
	procGetCursorPos                  = modUser32.NewProc("GetCursorPos")
	procLoadCursorW                   = modUser32.NewProc("LoadCursorW")
	procDestroyIcon                   = modUser32.NewProc("DestroyIcon")
	procGetDC                         = modUser32.NewProc("GetDC")
	procReleaseDC                     = modUser32.NewProc("ReleaseDC")
	procCreatePopupMenu               = modUser32.NewProc("CreatePopupMenu")
	procDestroyMenu                   = modUser32.NewProc("DestroyMenu")
	procAppendMenuW                   = modUser32.NewProc("AppendMenuW")
	procCheckMenuRadioItem            = modUser32.NewProc("CheckMenuRadioItem")
	procTrackPopupMenuEx              = modUser32.NewProc("TrackPopupMenuEx")
	procMonitorFromWindow             = modUser32.NewProc("MonitorFromWindow")
	procGetMonitorInfoW               = modUser32.NewProc("GetMonitorInfoW")
	procGetSystemMetrics              = modUser32.NewProc("GetSystemMetrics")
	procGetSystemMetricsForDpi        = modUser32.NewProc("GetSystemMetricsForDpi")
	procGetDpiForWindow               = modUser32.NewProc("GetDpiForWindow")
	procGetDpiForSystem               = modUser32.NewProc("GetDpiForSystem")
	procSetProcessDpiAwarenessContext = modUser32.NewProc("SetProcessDpiAwarenessContext")
)

const (
	singleInstanceMutexName = "Local\\MouseBattery.SingleInstance"
	hostClassName           = "MouseBatteryHost"

	wmNull          = 0x0000
	wmClose         = 0x0010
	wmEndSession    = 0x0016
	wmContextMenu   = 0x007B
	wmNCLButtonDown = 0x00A1
	wmLButtonDown   = 0x0201
	wmLButtonUp     = 0x0202
	wmRButtonUp     = 0x0205
	wmExitSizeMove  = 0x0232
	wmDpiChanged    = 0x02E0

	wmApp         = 0x8000
	wmAppTray     = wmApp + 1
	wmAppSnapshot = wmApp + 2
	wmAppActivate = wmApp + 3
)

type wndClassExW struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     uintptr
	hIcon         uintptr
	hCursor       uintptr
	hbrBackground uintptr
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       uintptr
}

type winPoint struct {
	X, Y int32
}

type winSize struct {
	CX, CY int32
}

type winRect struct {
	Left, Top, Right, Bottom int32
}

type monitorInfo struct {
	cbSize    uint32
	rcMonitor winRect
	rcWork    winRect
	dwFlags   uint32
}

type winMsg struct {
	HWnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      winPoint
}

var (
	activeWinApp *winApp
	hostWndProc  = syscall.NewCallback(hostProc)
	ballWndProc  = syscall.NewCallback(overlayProc)
)

type winApp struct {
	monitor        *battery.Monitor
	hostHWND       uintptr
	tray           *winTray
	overlay        *winOverlay
	taskbarCreated uint32
	state          battery.Snapshot
	view           ViewState

	mu       sync.Mutex
	pending  battery.Snapshot
	hasPend  bool
	quitting bool
}

func Run(monitor *battery.Monitor) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	mutexHandle, err := acquireSingleInstance()
	if err != nil {
		return err
	}
	if mutexHandle == 0 {
		// Another instance is already running and has been notified.
		return nil
	}
	defer windows.CloseHandle(mutexHandle)

	enablePerMonitorDPI()

	gdipToken, err := gdiplusStartup()
	if err != nil {
		return err
	}
	defer gdiplusShutdown(gdipToken)

	hInstance, _, _ := procGetModuleHandleW.Call(0)
	hostHWND, err := createHostWindow(hInstance)
	if err != nil {
		return err
	}

	taskbarMsgName, _ := syscall.UTF16PtrFromString("TaskbarCreated")
	taskbarCreated, _, _ := procRegisterWindowMessageW.Call(uintptr(unsafe.Pointer(taskbarMsgName)))

	overlay, err := newWinOverlay(hInstance, ballWndProc)
	if err != nil {
		procDestroyWindow.Call(hostHWND)
		return err
	}

	app := &winApp{
		monitor:        monitor,
		hostHWND:       hostHWND,
		tray:           newWinTray(hostHWND),
		overlay:        overlay,
		taskbarCreated: uint32(taskbarCreated),
	}
	activeWinApp = app
	defer func() {
		app.overlay.close()
		app.tray.close()
		procDestroyWindow.Call(app.hostHWND)
		activeWinApp = nil
	}()

	app.present(battery.Snapshot{Status: battery.Reading})
	app.overlay.setVisible(true)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go monitor.Run(ctx, func(s battery.Snapshot) {
		app.postSnapshot(s)
	})

	var msg winMsg
	for {
		ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(ret) <= 0 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
	cancel()
	return nil
}

func acquireSingleInstance() (windows.Handle, error) {
	name, err := windows.UTF16PtrFromString(singleInstanceMutexName)
	if err != nil {
		return 0, err
	}
	h, err := windows.CreateMutex(nil, false, name)
	if err != nil {
		if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
			if h != 0 {
				windows.CloseHandle(h)
			}
			classPtr, _ := syscall.UTF16PtrFromString(hostClassName)
			existing, _, _ := procFindWindowW.Call(uintptr(unsafe.Pointer(classPtr)), 0)
			if existing != 0 {
				procPostMessageW.Call(existing, wmAppActivate, 0, 0)
			}
			return 0, nil
		}
		return 0, fmt.Errorf("CreateMutex failed: %w", err)
	}
	return h, nil
}

func enablePerMonitorDPI() {
	if procSetProcessDpiAwarenessContext.Find() == nil {
		// DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 = (DPI_AWARENESS_CONTEXT)-4
		procSetProcessDpiAwarenessContext.Call(^uintptr(3))
	}
}

func systemDPI() uint32 {
	if procGetDpiForSystem.Find() == nil {
		if d, _, _ := procGetDpiForSystem.Call(); d > 0 {
			return uint32(d)
		}
	}
	return 96
}

func windowDPI(hwnd uintptr) uint32 {
	if hwnd != 0 && procGetDpiForWindow.Find() == nil {
		if d, _, _ := procGetDpiForWindow.Call(hwnd); d > 0 {
			return uint32(d)
		}
	}
	return systemDPI()
}

func createHostWindow(hInstance uintptr) (uintptr, error) {
	classPtr, _ := syscall.UTF16PtrFromString(hostClassName)
	titlePtr, _ := syscall.UTF16PtrFromString("Mouse Battery")
	wc := wndClassExW{
		cbSize:        uint32(unsafe.Sizeof(wndClassExW{})),
		lpfnWndProc:   hostWndProc,
		hInstance:     hInstance,
		lpszClassName: classPtr,
	}
	procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))

	// Use a hidden top-level window (not HWND_MESSAGE) so we receive TaskbarCreated broadcasts.
	hwnd, _, err := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(classPtr)),
		uintptr(unsafe.Pointer(titlePtr)),
		0,
		0, 0, 0, 0,
		0, 0, hInstance, 0,
	)
	if hwnd == 0 {
		return 0, fmt.Errorf("CreateWindowExW(host) failed: %w", err)
	}
	return hwnd, nil
}

func (a *winApp) postSnapshot(s battery.Snapshot) {
	a.mu.Lock()
	if a.quitting {
		a.mu.Unlock()
		return
	}
	a.pending = s
	a.hasPend = true
	a.mu.Unlock()
	procPostMessageW.Call(a.hostHWND, wmAppSnapshot, 0, 0)
}

func (a *winApp) drainSnapshot() {
	a.mu.Lock()
	if !a.hasPend {
		a.mu.Unlock()
		return
	}
	s := a.pending
	a.hasPend = false
	a.mu.Unlock()
	a.present(s)
}

func (a *winApp) present(s battery.Snapshot) {
	if s.Same(a.state) {
		return
	}
	a.state = s
	a.refreshView(true)
}

func (a *winApp) refreshView(redraw bool) {
	a.view = buildViewState(a.state, true, a.overlay.visible)
	if !redraw {
		return
	}
	_ = a.tray.update(a.view)
	_ = a.overlay.draw(a.view, nil)
}

func (a *winApp) setOverlayVisible(show bool) {
	if a.overlay.visible == show {
		return
	}
	a.overlay.setVisible(show)
	a.refreshView(false)
}

func (a *winApp) handleMenuCommand(items []MenuItem, cmd uint32) {
	item, ok := findMenuItem(items, cmd)
	if !ok {
		return
	}
	switch item.Action {
	case ActionSelect:
		a.monitor.Select(item.Device)
	case ActionRefresh:
		a.monitor.Refresh()
	case ActionToggleOverlay:
		a.setOverlayVisible(!a.overlay.visible)
	case ActionHideOverlay:
		a.setOverlayVisible(false)
	case ActionQuit:
		a.quit()
	}
}

func (a *winApp) quit() {
	a.mu.Lock()
	a.quitting = true
	a.mu.Unlock()
	procPostQuitMessage.Call(0)
}

func hostProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	app := activeWinApp
	if app == nil {
		r, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
		return r
	}
	if app.taskbarCreated != 0 && msg == app.taskbarCreated {
		app.tray.restore()
		return 0
	}
	switch msg {
	case wmAppSnapshot:
		app.drainSnapshot()
		return 0
	case wmAppActivate:
		app.setOverlayVisible(true)
		app.monitor.Refresh()
		return 0
	case wmAppTray:
		event := uint32(lParam & 0xFFFF)
		if event == wmLButtonUp || event == wmRButtonUp || event == wmContextMenu {
			items := app.view.TrayMenu
			cmd := showPopupMenu(app.hostHWND, items)
			app.handleMenuCommand(items, cmd)
		}
		return 0
	case wmEndSession:
		if wParam != 0 {
			app.quit()
		}
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
	return r
}

func overlayProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	app := activeWinApp
	if app == nil || app.overlay == nil {
		r, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
		return r
	}
	switch msg {
	case wmLButtonDown:
		app.overlay.beginDrag()
		return 0
	case wmRButtonUp:
		items := app.view.BallMenu
		cmd := showPopupMenu(app.hostHWND, items)
		app.handleMenuCommand(items, cmd)
		return 0
	case wmExitSizeMove:
		app.overlay.snapToWorkArea()
		return 0
	case wmDpiChanged:
		newDPI := uint32(wParam & 0xFFFF)
		var rc winRect
		var suggested *winRect
		if r, _, _ := procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&rc))); r != 0 {
			suggested = &rc
		}
		app.overlay.onDPIChanged(newDPI, suggested)
		// Taskbar DPI may also have changed; refresh tray icon too.
		_ = app.tray.update(app.view)
		return 0
	case wmClose:
		app.setOverlayVisible(false)
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
	return r
}
