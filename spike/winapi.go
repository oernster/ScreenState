//go:build windows

// Package main holds the ScreenState measurement spike. This file is the thin
// layer over the Windows interfaces the spike calls. It is throwaway code that
// exists to settle the open questions in REQUIREMENTS.md appendix B.
package main

import (
	"syscall"
	"unsafe"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	shcore   = syscall.NewLazyDLL("shcore.dll")
	dwmapi   = syscall.NewLazyDLL("dwmapi.dll")

	procEnumDisplayMonitors        = user32.NewProc("EnumDisplayMonitors")
	procGetMonitorInfoW            = user32.NewProc("GetMonitorInfoW")
	procEnumDisplayDevicesW        = user32.NewProc("EnumDisplayDevicesW")
	procEnumWindows                = user32.NewProc("EnumWindows")
	procGetWindowTextW             = user32.NewProc("GetWindowTextW")
	procGetClassNameW              = user32.NewProc("GetClassNameW")
	procGetWindowThreadProcessId   = user32.NewProc("GetWindowThreadProcessId")
	procIsWindowVisible            = user32.NewProc("IsWindowVisible")
	procIsWindow                   = user32.NewProc("IsWindow")
	procIsIconic                   = user32.NewProc("IsIconic")
	procIsZoomed                   = user32.NewProc("IsZoomed")
	procGetWindowRect              = user32.NewProc("GetWindowRect")
	procGetWindowPlacement         = user32.NewProc("GetWindowPlacement")
	procSetWindowPos               = user32.NewProc("SetWindowPos")
	procShowWindow                 = user32.NewProc("ShowWindow")
	procMonitorFromWindow          = user32.NewProc("MonitorFromWindow")
	procGetWindowLongPtrW          = user32.NewProc("GetWindowLongPtrW")
	procGetWindow                  = user32.NewProc("GetWindow")
	procPostMessageW               = user32.NewProc("PostMessageW")
	procSetForegroundWindow        = user32.NewProc("SetForegroundWindow")
	procSetProcessDpiAwarenessCtx  = user32.NewProc("SetProcessDpiAwarenessContext")
	procOpenProcess                = kernel32.NewProc("OpenProcess")
	procCloseHandle                = kernel32.NewProc("CloseHandle")
	procQueryFullProcessImageNameW = kernel32.NewProc("QueryFullProcessImageNameW")
	procGetExitCodeProcess         = kernel32.NewProc("GetExitCodeProcess")
	procGetApplicationUserModelID  = kernel32.NewProc("GetApplicationUserModelId")
	procGetTickCount64             = kernel32.NewProc("GetTickCount64")
	procGetDpiForMonitor           = shcore.NewProc("GetDpiForMonitor")
	procDwmGetWindowAttribute      = dwmapi.NewProc("DwmGetWindowAttribute")
	dpiAwarenessPerMonitorAwareV2  = ^uintptr(3) // -4 as an unsigned value
	processQueryLimitedInformation = uint32(0x1000)
)

const (
	monitorDefaultToNearest = 2
	eddGetDeviceInterface   = 1
	gwOwner                 = 4
	wsExToolWindow          = 0x00000080
	wsExAppWindow           = 0x00040000
	swRestore               = 9
	swMaximize              = 3
	swShow                  = 5
	swShowNormal            = 1
	swpNoZOrder             = 0x0004
	swpNoActivate           = 0x0010
	swHide                  = 0
	wmClose                 = 0x0010
	wmSysCommand            = 0x0112
	scClose                 = 0xF060
	dwmwaCloaked            = 14
	mdtEffectiveDpi         = 0
	maxTextLength           = 512
	stillActive             = 259
)

// RECT mirrors the Windows RECT structure.
type RECT struct {
	Left, Top, Right, Bottom int32
}

// Width answers the rectangle's width.
func (r RECT) Width() int32 { return r.Right - r.Left }

// Height answers the rectangle's height.
func (r RECT) Height() int32 { return r.Bottom - r.Top }

type point struct {
	X, Y int32
}

type windowPlacement struct {
	Length           uint32
	Flags            uint32
	ShowCmd          uint32
	PtMinPosition    point
	PtMaxPosition    point
	RcNormalPosition RECT
}

type monitorInfoEx struct {
	CbSize    uint32
	RcMonitor RECT
	RcWork    RECT
	DwFlags   uint32
	SzDevice  [32]uint16
}

type displayDevice struct {
	Cb           uint32
	DeviceName   [32]uint16
	DeviceString [128]uint16
	StateFlags   uint32
	DeviceID     [128]uint16
	DeviceKey    [128]uint16
}

func utf16ToString(buffer []uint16) string {
	for index, value := range buffer {
		if value == 0 {
			return syscall.UTF16ToString(buffer[:index])
		}
	}
	return syscall.UTF16ToString(buffer)
}

func adoptPerMonitorDpi() bool {
	if procSetProcessDpiAwarenessCtx.Find() != nil {
		return false
	}
	result, _, _ := procSetProcessDpiAwarenessCtx.Call(dpiAwarenessPerMonitorAwareV2)
	return result != 0
}

func tickCount() uint64 {
	value, _, _ := procGetTickCount64.Call()
	return uint64(value)
}

func monitorInfo(handle uintptr) (monitorInfoEx, bool) {
	var info monitorInfoEx
	info.CbSize = uint32(unsafe.Sizeof(info))
	result, _, _ := procGetMonitorInfoW.Call(handle, uintptr(unsafe.Pointer(&info)))
	return info, result != 0
}

func monitorDpi(handle uintptr) (uint32, uint32) {
	var dpiX, dpiY uint32
	if procGetDpiForMonitor.Find() != nil {
		return 0, 0
	}
	procGetDpiForMonitor.Call(
		handle,
		uintptr(mdtEffectiveDpi),
		uintptr(unsafe.Pointer(&dpiX)),
		uintptr(unsafe.Pointer(&dpiY)),
	)
	return dpiX, dpiY
}

func windowText(handle uintptr) string {
	buffer := make([]uint16, maxTextLength)
	procGetWindowTextW.Call(handle, uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)))
	return utf16ToString(buffer)
}

func windowClass(handle uintptr) string {
	buffer := make([]uint16, maxTextLength)
	procGetClassNameW.Call(handle, uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)))
	return utf16ToString(buffer)
}

func windowProcessID(handle uintptr) uint32 {
	var processID uint32
	procGetWindowThreadProcessId.Call(handle, uintptr(unsafe.Pointer(&processID)))
	return processID
}

func windowIsVisible(handle uintptr) bool {
	result, _, _ := procIsWindowVisible.Call(handle)
	return result != 0
}

func windowExists(handle uintptr) bool {
	result, _, _ := procIsWindow.Call(handle)
	return result != 0
}

func windowIsMinimised(handle uintptr) bool {
	result, _, _ := procIsIconic.Call(handle)
	return result != 0
}

func windowIsMaximised(handle uintptr) bool {
	result, _, _ := procIsZoomed.Call(handle)
	return result != 0
}

func windowIsCloaked(handle uintptr) bool {
	var cloaked uint32
	if procDwmGetWindowAttribute.Find() != nil {
		return false
	}
	procDwmGetWindowAttribute.Call(
		handle,
		uintptr(dwmwaCloaked),
		uintptr(unsafe.Pointer(&cloaked)),
		unsafe.Sizeof(cloaked),
	)
	return cloaked != 0
}

func windowRect(handle uintptr) RECT {
	var rect RECT
	procGetWindowRect.Call(handle, uintptr(unsafe.Pointer(&rect)))
	return rect
}

func windowPlacementOf(handle uintptr) windowPlacement {
	var placement windowPlacement
	placement.Length = uint32(unsafe.Sizeof(placement))
	procGetWindowPlacement.Call(handle, uintptr(unsafe.Pointer(&placement)))
	return placement
}

// gwlExStyle is the negative index GetWindowLongPtrW takes for the extended
// style. It is a variable rather than a constant because a negative constant
// cannot be converted to uintptr.
var gwlExStyle = int32(-20)

func windowExStyle(handle uintptr) uintptr {
	style, _, _ := procGetWindowLongPtrW.Call(handle, uintptr(gwlExStyle))
	return style
}

func windowOwner(handle uintptr) uintptr {
	owner, _, _ := procGetWindow.Call(handle, uintptr(gwOwner))
	return owner
}

func monitorOfWindow(handle uintptr) uintptr {
	monitor, _, _ := procMonitorFromWindow.Call(handle, uintptr(monitorDefaultToNearest))
	return monitor
}

func processImagePath(processID uint32) string {
	handle, _, _ := procOpenProcess.Call(
		uintptr(processQueryLimitedInformation),
		0,
		uintptr(processID),
	)
	if handle == 0 {
		return ""
	}
	defer procCloseHandle.Call(handle)
	buffer := make([]uint16, maxTextLength)
	size := uint32(len(buffer))
	result, _, _ := procQueryFullProcessImageNameW.Call(
		handle,
		0,
		uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(unsafe.Pointer(&size)),
	)
	if result == 0 {
		return ""
	}
	return utf16ToString(buffer[:size])
}

// processIsRunning answers whether a process still exists and has not exited.
// It is how the spike settles OQ-4: closing a window must leave the
// application running.
func processIsRunning(processID uint32) bool {
	handle, _, _ := procOpenProcess.Call(
		uintptr(processQueryLimitedInformation),
		0,
		uintptr(processID),
	)
	if handle == 0 {
		return false
	}
	defer procCloseHandle.Call(handle)
	var exitCode uint32
	result, _, _ := procGetExitCodeProcess.Call(handle, uintptr(unsafe.Pointer(&exitCode)))
	if result == 0 {
		return false
	}
	return exitCode == stillActive
}

// processApplicationUserModelID answers the packaged application identity of a
// process; an empty string where the process is not packaged. This is the
// candidate identity for Store applications such as Claude (OQ-2).
func processApplicationUserModelID(processID uint32) string {
	if procGetApplicationUserModelID.Find() != nil {
		return ""
	}
	handle, _, _ := procOpenProcess.Call(
		uintptr(processQueryLimitedInformation),
		0,
		uintptr(processID),
	)
	if handle == 0 {
		return ""
	}
	defer procCloseHandle.Call(handle)
	buffer := make([]uint16, maxTextLength)
	length := uint32(len(buffer))
	result, _, _ := procGetApplicationUserModelID.Call(
		handle,
		uintptr(unsafe.Pointer(&length)),
		uintptr(unsafe.Pointer(&buffer[0])),
	)
	if result != 0 {
		return ""
	}
	return utf16ToString(buffer[:length])
}
