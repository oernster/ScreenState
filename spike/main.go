//go:build windows

package main

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultLaunchWatch = 30 * time.Second
	minutesToDuration  = time.Minute
)

func main() {
	if len(os.Args) < 2 {
		usage(os.Stdout)
		return
	}
	command := strings.ToLower(os.Args[1])
	arguments := os.Args[2:]

	logFile, out := openLog(command)
	if logFile != nil {
		defer logFile.Close()
	}
	aware := adoptPerMonitorDpi()
	fmt.Fprintf(out, "ScreenState spike: %s\n", command)
	fmt.Fprintf(out, "Per-monitor DPI awareness adopted: %t\n", aware)
	fmt.Fprintf(out, "Started %s\n\n", time.Now().Format("2006-01-02 15:04:05"))

	switch command {
	case "displays":
		reportDisplays(out, enumerateDisplays())
	case "windows":
		all := len(arguments) > 0 && strings.EqualFold(arguments[0], "--all")
		reportWindows(out, enumerateWindows(), all)
	case "watch":
		runWatch(out, arguments)
	case "move":
		runMove(out, arguments)
	case "close":
		runHandleCommand(out, arguments, "close", closeWindow)
	case "sysclose":
		runHandleCommand(out, arguments, "sysclose", sysCloseWindow)
	case "hide":
		runHandleCommand(out, arguments, "hide", hideWindow)
	case "show":
		runHandleCommand(out, arguments, "show", showWindow)
	case "launch":
		runLaunch(out, arguments)
	default:
		fmt.Fprintf(out, "Unknown command: %s\n\n", command)
		usage(out)
	}
}

func runWatch(out io.Writer, arguments []string) {
	minutes := 10.0
	if len(arguments) > 0 {
		parsed, err := strconv.ParseFloat(arguments[0], 64)
		if err != nil {
			fmt.Fprintf(out, "watch takes a number of minutes. %q is not one.\n", arguments[0])
			return
		}
		minutes = parsed
	}
	watch(out, time.Duration(minutes*float64(minutesToDuration)))
}

func runMove(out io.Writer, arguments []string) {
	if len(arguments) < 2 {
		fmt.Fprintf(out, "move takes a window handle and a display index.\n")
		return
	}
	handle, ok := parseHandle(out, arguments[0])
	if !ok {
		return
	}
	index, err := strconv.Atoi(arguments[1])
	if err != nil {
		fmt.Fprintf(out, "%q is not a display index. Run the displays command first.\n", arguments[1])
		return
	}
	moveWindow(out, handle, index)
}

func runHandleCommand(out io.Writer, arguments []string, name string, action func(io.Writer, uintptr)) {
	if len(arguments) < 1 {
		fmt.Fprintf(out, "%s takes a window handle. Run the windows command first.\n", name)
		return
	}
	handle, ok := parseHandle(out, arguments[0])
	if !ok {
		return
	}
	action(out, handle)
}

func runLaunch(out io.Writer, arguments []string) {
	if len(arguments) < 1 {
		fmt.Fprintf(out, "launch takes a command and any arguments for it.\n")
		return
	}
	launchAndWatch(out, arguments[0], arguments[1:], defaultLaunchWatch)
}

func parseHandle(out io.Writer, text string) (uintptr, bool) {
	cleaned := strings.TrimPrefix(strings.TrimPrefix(text, "0x"), "0X")
	base := 16
	if cleaned == text {
		base = 10
	}
	value, err := strconv.ParseUint(cleaned, base, 64)
	if err != nil {
		fmt.Fprintf(out, "%q is not a window handle. Run the windows command first.\n", text)
		return 0, false
	}
	return uintptr(value), true
}

func openLog(command string) (*os.File, io.Writer) {
	name := fmt.Sprintf("spike-%s-%s.log", command, time.Now().Format("20060102-150405"))
	file, err := os.Create(name)
	if err != nil {
		fmt.Printf("Writing no log file: %v\n", err)
		return nil, os.Stdout
	}
	fmt.Printf("Log: %s\n\n", name)
	return file, io.MultiWriter(os.Stdout, file)
}

func usage(out io.Writer) {
	fmt.Fprintf(out, `ScreenState measurement spike.

Every command writes what it prints to a log file beside the executable.

  displays              Read every identifier for every connected display.
                        Run before a reboot and after one, then compare. (OQ-1)

  windows [--all]       Read every identifier for every top-level window.
                        Without --all, only windows a user would call a
                        window of an application. (OQ-2)

  move <hwnd> <index>   Move a window to a display and maximise it there,
                        then read back where it went. (OQ-3)

  close <hwnd>          Ask a window to close, then report whether the
                        application is still running. (OQ-4)

  show <hwnd>           Try to make a hidden window visible. (OQ-6)

  launch <cmd> [args]   Start a command, then report every window that
                        appears over the next 30 seconds. Running an
                        application that is already running is how a
                        tray-only application is usually asked to show
                        itself. (OQ-6)

  watch [minutes]       Record every window that appears, vanishes, moves or
                        changes state. Default 10 minutes. Run it at sign-in
                        to measure the real startup. (OQ-5, OQ-7)

`)
}
