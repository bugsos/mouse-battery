//go:build windows

package ui

import (
	"fmt"
	"math"
	"syscall"
	"unsafe"
)

const (
	overlayClassName = "MouseBatteryOverlay"

	wsPopup          = 0x80000000
	wsExTopmost      = 0x00000008
	wsExToolWindow   = 0x00000080
	wsExLayered      = 0x00080000
	wsExNoActivate   = 0x08000000
	ulwAlpha         = 0x00000002
	acSrcOver        = 0x00
	acSrcAlpha       = 0x01
	htCaption        = 2
	swHide           = 0
	swShowNoActivate = 4

	swpNoSize     = 0x0001
	swpNoMove     = 0x0002
	swpNoActivate = 0x0010
	swpShowWindow = 0x0040

	monitorDefaultToPrimary = 1
	monitorDefaultToNearest = 2

	idcArrow = 32512
)

var hwndTopmost = ^uintptr(0) // HWND_TOPMOST = (HWND)-1

type blendFunction struct {
	BlendOp             byte
	BlendFlags          byte
	SourceConstantAlpha byte
	AlphaFormat         byte
}

type winOverlay struct {
	hwnd    uintptr
	dpi     uint32
	visible bool
	placed  bool
	view    ViewState
}

func newWinOverlay(hInstance uintptr, wndProc uintptr) (*winOverlay, error) {
	classPtr, _ := syscall.UTF16PtrFromString(overlayClassName)
	titlePtr, _ := syscall.UTF16PtrFromString("鼠标电量")
	cursor, _, _ := procLoadCursorW.Call(0, idcArrow)

	wc := wndClassExW{
		cbSize:        uint32(unsafe.Sizeof(wndClassExW{})),
		lpfnWndProc:   wndProc,
		hInstance:     hInstance,
		hCursor:       cursor,
		lpszClassName: classPtr,
	}
	procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))

	exStyle := uintptr(wsExLayered | wsExTopmost | wsExToolWindow | wsExNoActivate)
	hwnd, _, err := procCreateWindowExW.Call(
		exStyle,
		uintptr(unsafe.Pointer(classPtr)),
		uintptr(unsafe.Pointer(titlePtr)),
		wsPopup,
		0, 0, 56, 28,
		0, 0, hInstance, 0,
	)
	if hwnd == 0 {
		return nil, fmt.Errorf("CreateWindowExW(overlay) failed: %w", err)
	}

	o := &winOverlay{
		hwnd:    hwnd,
		dpi:     windowDPI(hwnd),
		visible: true,
	}
	return o, nil
}

func (o *winOverlay) draw(vs ViewState, dst *winPoint) error {
	o.view = vs
	surf, err := renderOverlayDIB(vs, o.dpi)
	if err != nil {
		return err
	}
	defer surf.Close()

	if !o.placed && dst == nil {
		pt := o.initialPosition(surf.width, surf.height)
		dst = &pt
		o.placed = true
	}

	size := winSize{CX: surf.width, CY: surf.height}
	src := winPoint{X: 0, Y: 0}
	blend := blendFunction{
		BlendOp:             acSrcOver,
		SourceConstantAlpha: 255,
		AlphaFormat:         acSrcAlpha,
	}

	var dstPtr uintptr
	if dst != nil {
		dstPtr = uintptr(unsafe.Pointer(dst))
	}
	r, _, err := procUpdateLayeredWindow.Call(
		o.hwnd,
		0,
		dstPtr,
		uintptr(unsafe.Pointer(&size)),
		surf.hdc,
		uintptr(unsafe.Pointer(&src)),
		0,
		uintptr(unsafe.Pointer(&blend)),
		ulwAlpha,
	)
	if r == 0 {
		return fmt.Errorf("UpdateLayeredWindow failed: %w", err)
	}
	return nil
}

func (o *winOverlay) initialPosition(width, height int32) winPoint {
	mon, _, _ := procMonitorFromWindow.Call(0, monitorDefaultToPrimary)
	if mon != 0 {
		var mi monitorInfo
		mi.cbSize = uint32(unsafe.Sizeof(mi))
		if r, _, _ := procGetMonitorInfoW.Call(mon, uintptr(unsafe.Pointer(&mi))); r != 0 {
			x := mi.rcWork.Right - width - int32(scaleDIP(8, o.dpi))
			y := mi.rcWork.Bottom - height - int32(scaleDIP(32, o.dpi))
			return winPoint{X: x, Y: y}
		}
	}
	return winPoint{X: 100, Y: 100}
}

func (o *winOverlay) setVisible(show bool) {
	o.visible = show
	if show {
		procSetWindowPos.Call(o.hwnd, hwndTopmost, 0, 0, 0, 0, swpNoMove|swpNoSize|swpNoActivate|swpShowWindow)
		procShowWindow.Call(o.hwnd, swShowNoActivate)
	} else {
		procShowWindow.Call(o.hwnd, swHide)
	}
}

func (o *winOverlay) beginDrag() {
	procReleaseCapture.Call()
	procSendMessageW.Call(o.hwnd, wmNCLButtonDown, htCaption, 0)
}

func (o *winOverlay) snapToWorkArea() {
	var rc winRect
	if r, _, _ := procGetWindowRect.Call(o.hwnd, uintptr(unsafe.Pointer(&rc))); r == 0 {
		return
	}
	mon, _, _ := procMonitorFromWindow.Call(o.hwnd, monitorDefaultToNearest)
	if mon == 0 {
		return
	}
	var mi monitorInfo
	mi.cbSize = uint32(unsafe.Sizeof(mi))
	if r, _, _ := procGetMonitorInfoW.Call(mon, uintptr(unsafe.Pointer(&mi))); r == 0 {
		return
	}

	w := int(rc.Right - rc.Left)
	h := int(rc.Bottom - rc.Top)
	threshold := scaleDIP(12, o.dpi)
	x := snapAxis(int(rc.Left), w, int(mi.rcWork.Left), int(mi.rcWork.Right-mi.rcWork.Left), threshold)
	y := snapAxis(int(rc.Top), h, int(mi.rcWork.Top), int(mi.rcWork.Bottom-mi.rcWork.Top), threshold)
	if x != int(rc.Left) || y != int(rc.Top) {
		procSetWindowPos.Call(o.hwnd, hwndTopmost, uintptr(x), uintptr(y), 0, 0, swpNoSize|swpNoActivate)
	}
}

func (o *winOverlay) onDPIChanged(newDPI uint32, suggested *winRect) {
	if newDPI == 0 {
		newDPI = 96
	}
	o.dpi = newDPI
	var dst *winPoint
	if suggested != nil {
		dst = &winPoint{X: suggested.Left, Y: suggested.Top}
	}
	_ = o.draw(o.view, dst)
}

func (o *winOverlay) close() {
	if o.hwnd != 0 {
		procDestroyWindow.Call(o.hwnd)
		o.hwnd = 0
	}
}

func scaleDIP(dip int, dpi uint32) int {
	if dpi == 0 {
		dpi = 96
	}
	return int(math.Round(float64(dip) * float64(dpi) / 96.0))
}
