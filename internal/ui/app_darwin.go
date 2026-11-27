//go:build darwin

package ui

/*
#cgo OBJCFLAGS: -fobjc-arc
#cgo LDFLAGS: -framework Cocoa
#include <stdlib.h>
#include "darwin_cocoa.h"
*/
import "C"

import (
	"context"
	"runtime"
	"sync"
	"unsafe"

	"mouse-battery/internal/battery"
)

func init() {
	// AppKit requires the main goroutine to stay on OS thread 0.
	runtime.LockOSThread()
}

type darwinApp struct {
	monitor *battery.Monitor
	ctx     context.Context
	cancel  context.CancelFunc

	mu        sync.Mutex
	autoStart bool
	state     battery.Snapshot
	view      ViewState
}

var (
	darwinMu  sync.Mutex
	activeMac *darwinApp
)

func Run(monitor *battery.Monitor) error {
	ctx, cancel := context.WithCancel(context.Background())
	app := &darwinApp{
		monitor:   monitor,
		ctx:       ctx,
		cancel:    cancel,
		autoStart: isAutoStartEnabled(),
	}

	darwinMu.Lock()
	activeMac = app
	darwinMu.Unlock()

	defer func() {
		cancel()
		darwinMu.Lock()
		activeMac = nil
		darwinMu.Unlock()
	}()

	C.mb_run()
	return nil
}

//export goDarwinReady
func goDarwinReady() {
	darwinMu.Lock()
	app := activeMac
	darwinMu.Unlock()
	if app == nil {
		return
	}
	app.present(battery.Snapshot{Status: battery.Reading})
	go app.monitor.Run(app.ctx, func(s battery.Snapshot) {
		if app.ctx.Err() == nil {
			app.present(s)
		}
	})
}

//export goDarwinMenuClick
func goDarwinMenuClick(itemID C.uint32_t) {
	darwinMu.Lock()
	app := activeMac
	darwinMu.Unlock()
	if app == nil {
		return
	}

	app.mu.Lock()
	items := app.view.TrayMenu
	app.mu.Unlock()

	item, ok := findMenuItem(items, uint32(itemID))
	if !ok {
		return
	}
	switch item.Action {
	case ActionSelect:
		app.monitor.Select(item.Device)
	case ActionRefresh:
		app.monitor.Refresh()
	case ActionToggleAutoStart:
		app.toggleAutoStart()
	case ActionQuit:
		app.cancel()
		C.mb_quit()
	}
}

func (a *darwinApp) toggleAutoStart() {
	a.mu.Lock()
	next := !a.autoStart
	a.mu.Unlock()

	if err := setAutoStart(next); err != nil {
		next = isAutoStartEnabled()
	}

	a.mu.Lock()
	a.autoStart = next
	vs := buildViewState(a.state, uiOptions{hasAutoStart: true, autoStart: a.autoStart})
	a.view = vs
	a.mu.Unlock()

	a.pushView(vs)
}

func (a *darwinApp) present(s battery.Snapshot) {
	a.mu.Lock()
	if s.Same(a.state) {
		a.mu.Unlock()
		return
	}
	a.state = s
	vs := buildViewState(s, uiOptions{hasAutoStart: true, autoStart: a.autoStart})
	a.view = vs
	a.mu.Unlock()

	a.pushView(vs)
}

func (a *darwinApp) pushView(vs ViewState) {
	cTitle := C.CString(vs.BadgeText)
	cTooltip := C.CString(vs.Tooltip)
	defer C.free(unsafe.Pointer(cTitle))
	defer C.free(unsafe.Pointer(cTooltip))

	n := len(vs.TrayMenu)
	if n == 0 {
		C.mb_update(cTitle, cTooltip, nil, 0)
		return
	}

	cItems := make([]C.mb_menu_item, n)
	cLabels := make([]*C.char, 0, n)
	defer func() {
		for _, p := range cLabels {
			C.free(unsafe.Pointer(p))
		}
	}()

	for i, item := range vs.TrayMenu {
		var lbl *C.char
		if item.Label != "" {
			lbl = C.CString(item.Label)
			cLabels = append(cLabels, lbl)
		}
		var checked C.uint8_t
		if item.Checked {
			checked = 1
		}
		cItems[i] = C.mb_menu_item{
			id:      C.uint32_t(item.ID),
			kind:    C.uint8_t(item.Kind),
			checked: checked,
			label:   lbl,
		}
	}

	C.mb_update(cTitle, cTooltip, &cItems[0], C.int(n))
}
