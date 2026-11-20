//go:build windows

package ui

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modShell32 = windows.NewLazySystemDLL("shell32.dll")

	procShellNotifyIconW = modShell32.NewProc("Shell_NotifyIconW")
)

const (
	nimAdd    = 0x00000000
	nimModify = 0x00000001
	nimDelete = 0x00000002

	nifMessage = 0x00000001
	nifIcon    = 0x00000002
	nifTip     = 0x00000004

	mfByCommand = 0x00000000
	mfString    = 0x00000000
	mfGrayed    = 0x00000001
	mfDisabled  = 0x00000002
	mfChecked   = 0x00000008
	mfSeparator = 0x00000800

	tpmRightButton = 0x0002
	tpmNoNotify    = 0x0080
	tpmReturnCmd   = 0x0100

	smCxSmIcon = 49
)

type notifyIconDataW struct {
	cbSize           uint32
	hWnd             uintptr
	uID              uint32
	uFlags           uint32
	uCallbackMessage uint32
	hIcon            uintptr
	szTip            [128]uint16
	dwState          uint32
	dwStateMask      uint32
	szInfo           [256]uint16
	uVersion         uint32
	szInfoTitle      [64]uint16
	dwInfoFlags      uint32
	guidItem         windows.GUID
	hBalloonIcon     uintptr
}

type winTray struct {
	hwnd    uintptr
	icon    uintptr
	tooltip string
	added   bool
}

func newWinTray(hwnd uintptr) *winTray {
	return &winTray{hwnd: hwnd}
}

func trayIconPixelSize() int32 {
	dpi := systemDPI()
	if procGetSystemMetricsForDpi.Find() == nil {
		if r, _, _ := procGetSystemMetricsForDpi.Call(smCxSmIcon, uintptr(dpi)); r > 0 {
			return int32(r)
		}
	}
	if r, _, _ := procGetSystemMetrics.Call(smCxSmIcon); r > 0 {
		return int32(r)
	}
	return 16
}

func (t *winTray) update(vs ViewState) error {
	hicon, err := renderTrayHICON(vs, trayIconPixelSize())
	if err != nil {
		return err
	}
	prev := t.icon
	t.icon = hicon
	t.tooltip = vs.Tooltip

	if err := t.apply(); err != nil {
		if prev != 0 {
			procDestroyIcon.Call(hicon)
			t.icon = prev
		}
		return err
	}
	if prev != 0 {
		procDestroyIcon.Call(prev)
	}
	return nil
}

func (t *winTray) restore() {
	if t.icon == 0 {
		return
	}
	t.added = false
	_ = t.apply()
}

func (t *winTray) apply() error {
	var nid notifyIconDataW
	nid.cbSize = uint32(unsafe.Sizeof(nid))
	nid.hWnd = t.hwnd
	nid.uID = 1
	nid.uFlags = nifMessage | nifIcon | nifTip
	nid.uCallbackMessage = wmAppTray
	nid.hIcon = t.icon
	copyUTF16Truncated(nid.szTip[:], t.tooltip)

	msg := uintptr(nimModify)
	if !t.added {
		msg = nimAdd
	}
	r, _, _ := procShellNotifyIconW.Call(msg, uintptr(unsafe.Pointer(&nid)))
	if r == 0 && t.added {
		// Explorer may have restarted between updates; retry with NIM_ADD.
		r, _, _ = procShellNotifyIconW.Call(nimAdd, uintptr(unsafe.Pointer(&nid)))
	}
	if r == 0 {
		return fmt.Errorf("Shell_NotifyIconW failed")
	}
	t.added = true
	return nil
}

func (t *winTray) close() {
	if t.added {
		var nid notifyIconDataW
		nid.cbSize = uint32(unsafe.Sizeof(nid))
		nid.hWnd = t.hwnd
		nid.uID = 1
		procShellNotifyIconW.Call(nimDelete, uintptr(unsafe.Pointer(&nid)))
		t.added = false
	}
	if t.icon != 0 {
		procDestroyIcon.Call(t.icon)
		t.icon = 0
	}
}

func copyUTF16Truncated(dst []uint16, s string) {
	if len(dst) == 0 {
		return
	}
	u16, err := syscall.UTF16FromString(s)
	if err != nil {
		return
	}
	if len(u16) > len(dst) {
		u16 = u16[:len(dst)]
		u16[len(u16)-1] = 0
	}
	copy(dst, u16)
}

func showPopupMenu(hwnd uintptr, items []MenuItem) uint32 {
	hMenu, _, _ := procCreatePopupMenu.Call()
	if hMenu == 0 {
		return 0
	}
	defer procDestroyMenu.Call(hMenu)

	var radioFirst, radioLast, radioChecked uint32
	flushRadios := func() {
		if radioFirst != 0 && radioChecked != 0 {
			procCheckMenuRadioItem.Call(hMenu, uintptr(radioFirst), uintptr(radioLast), uintptr(radioChecked), mfByCommand)
		}
		radioFirst, radioLast, radioChecked = 0, 0, 0
	}

	for _, item := range items {
		if item.Kind != MenuRadio {
			flushRadios()
		}
		switch item.Kind {
		case MenuSeparator:
			procAppendMenuW.Call(hMenu, mfSeparator, 0, 0)
		case MenuLabel:
			ptr, _ := syscall.UTF16PtrFromString(item.Label)
			procAppendMenuW.Call(hMenu, mfString|mfGrayed|mfDisabled, uintptr(item.ID), uintptr(unsafe.Pointer(ptr)))
		case MenuRadio:
			ptr, _ := syscall.UTF16PtrFromString(item.Label)
			procAppendMenuW.Call(hMenu, mfString, uintptr(item.ID), uintptr(unsafe.Pointer(ptr)))
			if radioFirst == 0 {
				radioFirst = item.ID
			}
			radioLast = item.ID
			if item.Checked {
				radioChecked = item.ID
			}
		case MenuCheck:
			flags := uintptr(mfString)
			if item.Checked {
				flags |= mfChecked
			}
			ptr, _ := syscall.UTF16PtrFromString(item.Label)
			procAppendMenuW.Call(hMenu, flags, uintptr(item.ID), uintptr(unsafe.Pointer(ptr)))
		case MenuAction:
			ptr, _ := syscall.UTF16PtrFromString(item.Label)
			procAppendMenuW.Call(hMenu, mfString, uintptr(item.ID), uintptr(unsafe.Pointer(ptr)))
		}
	}
	flushRadios()

	var pt winPoint
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	procSetForegroundWindow.Call(hwnd)
	cmd, _, _ := procTrackPopupMenuEx.Call(
		hMenu,
		tpmRightButton|tpmReturnCmd|tpmNoNotify,
		uintptr(pt.X),
		uintptr(pt.Y),
		hwnd,
		0,
	)
	procPostMessageW.Call(hwnd, wmNull, 0, 0)
	return uint32(cmd)
}
