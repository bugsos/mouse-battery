// Command probe lists and reads every supported device without starting the desktop UI.
package main

import (
	"context"
	"fmt"
	"os"

	"mouse-battery/internal/drivers"
)

func main() {
	ctx := context.Background()
	devices, err := drivers.Discover(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	if len(devices) == 0 {
		fmt.Println("未发现已识别的鼠标")
		os.Exit(1)
	}
	failed := err != nil
	for _, device := range devices {
		value, readErr := device.ReadBattery(ctx)
		if readErr != nil {
			fmt.Fprintf(os.Stderr, "%s [%s] · 暂不可读 (%v)\n", device.Name, device.Connection, readErr)
			failed = true
			continue
		}
		fmt.Printf("%s [%s] · %d%%\n", device.Name, device.Connection, value)
	}
	if failed {
		os.Exit(1)
	}
}
