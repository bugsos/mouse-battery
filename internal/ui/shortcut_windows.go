//go:build windows

package ui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modOle32 = windows.NewLazySystemDLL("ole32.dll")

	procCoInitializeEx       = modOle32.NewProc("CoInitializeEx")
	procCoUninitialize       = modOle32.NewProc("CoUninitialize")
	procCoCreateInstance     = modOle32.NewProc("CoCreateInstance")
	procCoTaskMemFree        = modOle32.NewProc("CoTaskMemFree")
	procSHGetKnownFolderPath = modShell32.NewProc("SHGetKnownFolderPath")
)

const (
	shortcutFileName = "Mouse Battery.lnk"

	coinitApartmentThreaded = 0x2
	clsctxInprocServer      = 0x1
	rpcEChangedMode         = 0x80010106
	stgmRead                = 0x0
)

var (
	// FOLDERID_Desktop {B4BFCC3A-DB2C-424C-B029-7FE99A87C641}
	folderIDDesktop = windows.GUID{
		Data1: 0xB4BFCC3A,
		Data2: 0xDB2C,
		Data3: 0x424C,
		Data4: [8]byte{0xB0, 0x29, 0x7F, 0xE9, 0x9A, 0x87, 0xC6, 0x41},
	}
	// CLSID_ShellLink {00021401-0000-0000-C000-000000000046}
	clsidShellLink = windows.GUID{
		Data1: 0x00021401,
		Data4: [8]byte{0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46},
	}
	// IID_IShellLinkW {000214F9-0000-0000-C000-000000000046}
	iidIShellLinkW = windows.GUID{
		Data1: 0x000214F9,
		Data4: [8]byte{0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46},
	}
	// IID_IPersistFile {0000010B-0000-0000-C000-000000000046}
	iidIPersistFile = windows.GUID{
		Data1: 0x0000010B,
		Data4: [8]byte{0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46},
	}
)

type comObject struct {
	vtbl *[32]uintptr
}

func (o *comObject) release() {
	if o != nil {
		syscall.SyscallN(o.vtbl[2], uintptr(unsafe.Pointer(o)))
	}
}

func (o *comObject) queryInterface(iid *windows.GUID) (*comObject, error) {
	var out *comObject
	hr, _, _ := syscall.SyscallN(
		o.vtbl[0],
		uintptr(unsafe.Pointer(o)),
		uintptr(unsafe.Pointer(iid)),
		uintptr(unsafe.Pointer(&out)),
	)
	if int32(hr) < 0 || out == nil {
		return nil, fmt.Errorf("QueryInterface failed: 0x%08x", uint32(hr))
	}
	return out, nil
}

func withCOM(fn func() error) error {
	hr, _, _ := procCoInitializeEx.Call(0, coinitApartmentThreaded)
	code := uint32(hr)
	if int32(hr) < 0 && code != rpcEChangedMode {
		return fmt.Errorf("CoInitializeEx failed: 0x%08x", code)
	}
	if code == 0 || code == 1 {
		defer procCoUninitialize.Call()
	}
	return fn()
}

func desktopDir() (string, error) {
	var p *uint16
	hr, _, _ := procSHGetKnownFolderPath.Call(
		uintptr(unsafe.Pointer(&folderIDDesktop)),
		0,
		0,
		uintptr(unsafe.Pointer(&p)),
	)
	if int32(hr) < 0 || p == nil {
		return "", fmt.Errorf("SHGetKnownFolderPath(Desktop) failed: 0x%08x", uint32(hr))
	}
	defer procCoTaskMemFree.Call(uintptr(unsafe.Pointer(p)))
	return windows.UTF16PtrToString(p), nil
}

func isDesktopShortcutEnabled() bool {
	dir, err := desktopDir()
	if err != nil {
		return false
	}
	exe, err := currentExecutablePath()
	if err != nil {
		return false
	}
	return isShortcutPresentInDir(dir, exe)
}

func setDesktopShortcut(enable bool) error {
	dir, err := desktopDir()
	if err != nil {
		return err
	}
	exe, err := currentExecutablePath()
	if err != nil {
		return err
	}
	return setShortcutInDir(dir, exe, enable)
}

func isShortcutPresentInDir(dir, exe string) bool {
	lnkPath := filepath.Join(dir, shortcutFileName)
	if _, err := os.Stat(lnkPath); err != nil {
		return false
	}
	target, err := readShellLinkTarget(lnkPath)
	if err != nil || target == "" {
		return false
	}
	return strings.EqualFold(filepath.Clean(target), filepath.Clean(exe))
}

func setShortcutInDir(dir, exe string, enable bool) error {
	lnkPath := filepath.Join(dir, shortcutFileName)
	if !enable {
		if err := os.Remove(lnkPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	return createShellLink(lnkPath, exe)
}

func createShellLink(lnkPath, targetExe string) error {
	return withCOM(func() error {
		var sl *comObject
		hr, _, _ := procCoCreateInstance.Call(
			uintptr(unsafe.Pointer(&clsidShellLink)),
			0,
			clsctxInprocServer,
			uintptr(unsafe.Pointer(&iidIShellLinkW)),
			uintptr(unsafe.Pointer(&sl)),
		)
		if int32(hr) < 0 || sl == nil {
			return fmt.Errorf("CoCreateInstance(ShellLink) failed: 0x%08x", uint32(hr))
		}
		defer sl.release()

		cleanExe := filepath.Clean(targetExe)
		exePtr, err := syscall.UTF16PtrFromString(cleanExe)
		if err != nil {
			return err
		}
		dirPtr, err := syscall.UTF16PtrFromString(filepath.Dir(cleanExe))
		if err != nil {
			return err
		}
		descPtr, err := syscall.UTF16PtrFromString("Mouse Battery")
		if err != nil {
			return err
		}
		lnkPtr, err := syscall.UTF16PtrFromString(lnkPath)
		if err != nil {
			return err
		}

		// IShellLinkW::SetPath (vtable slot 20)
		if hr, _, _ := syscall.SyscallN(sl.vtbl[20], uintptr(unsafe.Pointer(sl)), uintptr(unsafe.Pointer(exePtr))); int32(hr) < 0 {
			return fmt.Errorf("IShellLinkW::SetPath failed: 0x%08x", uint32(hr))
		}
		// IShellLinkW::SetWorkingDirectory (vtable slot 9)
		_, _, _ = syscall.SyscallN(sl.vtbl[9], uintptr(unsafe.Pointer(sl)), uintptr(unsafe.Pointer(dirPtr)))
		// IShellLinkW::SetDescription (vtable slot 7)
		_, _, _ = syscall.SyscallN(sl.vtbl[7], uintptr(unsafe.Pointer(sl)), uintptr(unsafe.Pointer(descPtr)))
		// IShellLinkW::SetIconLocation (vtable slot 17) -> embedded icon index 0
		_, _, _ = syscall.SyscallN(sl.vtbl[17], uintptr(unsafe.Pointer(sl)), uintptr(unsafe.Pointer(exePtr)), 0)

		pf, err := sl.queryInterface(&iidIPersistFile)
		if err != nil {
			return err
		}
		defer pf.release()

		// IPersistFile::Save (vtable slot 6)
		if hr, _, _ := syscall.SyscallN(pf.vtbl[6], uintptr(unsafe.Pointer(pf)), uintptr(unsafe.Pointer(lnkPtr)), 1); int32(hr) < 0 {
			return fmt.Errorf("IPersistFile::Save failed: 0x%08x", uint32(hr))
		}
		return nil
	})
}

func readShellLinkTarget(lnkPath string) (string, error) {
	var result string
	err := withCOM(func() error {
		var sl *comObject
		hr, _, _ := procCoCreateInstance.Call(
			uintptr(unsafe.Pointer(&clsidShellLink)),
			0,
			clsctxInprocServer,
			uintptr(unsafe.Pointer(&iidIShellLinkW)),
			uintptr(unsafe.Pointer(&sl)),
		)
		if int32(hr) < 0 || sl == nil {
			return fmt.Errorf("CoCreateInstance(ShellLink) failed: 0x%08x", uint32(hr))
		}
		defer sl.release()

		pf, err := sl.queryInterface(&iidIPersistFile)
		if err != nil {
			return err
		}
		defer pf.release()

		lnkPtr, err := syscall.UTF16PtrFromString(lnkPath)
		if err != nil {
			return err
		}
		// IPersistFile::Load (vtable slot 5)
		if hr, _, _ := syscall.SyscallN(pf.vtbl[5], uintptr(unsafe.Pointer(pf)), uintptr(unsafe.Pointer(lnkPtr)), stgmRead); int32(hr) < 0 {
			return fmt.Errorf("IPersistFile::Load failed: 0x%08x", uint32(hr))
		}

		// IShellLinkW::GetPath (vtable slot 3)
		buf := make([]uint16, 1024)
		if hr, _, _ := syscall.SyscallN(
			sl.vtbl[3],
			uintptr(unsafe.Pointer(sl)),
			uintptr(unsafe.Pointer(&buf[0])),
			uintptr(len(buf)),
			0,
			0,
		); int32(hr) < 0 {
			return fmt.Errorf("IShellLinkW::GetPath failed: 0x%08x", uint32(hr))
		}
		result = syscall.UTF16ToString(buf)
		return nil
	})
	return result, err
}
