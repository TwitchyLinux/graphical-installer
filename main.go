package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/gotk3/gotk3/gtk"
)

var (
	debugMode = flag.Bool("debug", false, "Enable debugging")
	version   = flag.String("version", "", "Version to display")
)

func main() {
	flag.Parse()
	args := flag.Args()
	var err error
	defer func() {
		if err != nil {
			fmt.Fprintf(os.Stderr, "Fatal error!: %v\n", err)
			os.Exit(1)
		}
	}()

	gtk.Init(&args)

	mw, err := makeMainWindow()
	if err != nil {
		return
	}

	readDiskInfo(mw)
	readNetInfo(mw)
	readTimezoneInfo(mw)

	mw.mainLoop()
}
