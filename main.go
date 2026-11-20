package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"mouse-battery/internal/battery"
	"mouse-battery/internal/drivers"
	"mouse-battery/internal/ui"
)

func main() {
	once := flag.Bool("read-once", false, "打印一次当前设备和电量")
	flag.Parse()
	monitor := battery.NewMonitor(drivers.Discover)
	if *once {
		state, err := monitor.ReadOnce(context.Background())
		fmt.Println(ui.Description(state))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		if state.Status != battery.Ready {
			os.Exit(1)
		}
		return
	}
	if err := ui.Run(monitor); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
