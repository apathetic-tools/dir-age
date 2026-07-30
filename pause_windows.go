//go:build windows

package main

import (
	"bufio"
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32                  = windows.NewLazySystemDLL("kernel32.dll")
	procGetConsoleProcessList = kernel32.NewProc("GetConsoleProcessList")
)

// pauseIfDoubleClicked waits for a keypress before exiting, but only when
// this process owns its console window (i.e. it was launched by double-click
// or drag-and-drop from Explorer, which would otherwise close the window
// immediately). When run from an existing terminal, the console has other
// attached processes and we exit immediately as a normal CLI tool would.
func pauseIfDoubleClicked() {
	var procList [2]uint32
	ret, _, _ := procGetConsoleProcessList.Call(
		uintptr(unsafe.Pointer(&procList[0])),
		uintptr(len(procList)),
	)
	if ret > 1 {
		return
	}
	fmt.Print("\nPress Enter to exit...")
	bufio.NewReader(os.Stdin).ReadString('\n')
}
