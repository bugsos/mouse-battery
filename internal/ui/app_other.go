//go:build !windows && !darwin

package ui

import (
	"fmt"
	"runtime"

	"mouse-battery/internal/battery"
)

func Run(*battery.Monitor) error {
	return fmt.Errorf("desktop UI is not yet implemented for %s (use -read-once for CLI output)", runtime.GOOS)
}
