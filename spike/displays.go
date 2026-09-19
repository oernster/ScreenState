//go:build windows

package main

import (
	"fmt"
	"io"
	"syscall"
	"unsafe"
)

// displayRecord holds every identifier the spike can read for one display.
// OQ-1 asks which of these survives a reboot and tells identical models apart.
type displayRecord struct {
	Handle          uintptr
	GDIName         string
	Primary         bool
	Bounds          RECT
	WorkArea        RECT
	DpiX            uint32
	DpiY            uint32
	AdapterName     string
	AdapterDeviceID string
	AdapterKey      string
	MonitorName     string
	MonitorDeviceID string
	MonitorKey      string
}

func enumerateDisplays() []displayRecord {
	var records []displayRecord
	callback := syscall.NewCallback(func(monitor uintptr, _ uintptr, _ uintptr, _ uintptr) uintptr {
		info, ok := monitorInfo(monitor)
		if !ok {
			return 1
		}
		dpiX, dpiY := monitorDpi(monitor)
		record := displayRecord{
			Handle:   monitor,
			GDIName:  utf16ToString(info.SzDevice[:]),
			Primary:  info.DwFlags&1 != 0,
			Bounds:   info.RcMonitor,
			WorkArea: info.RcWork,
			DpiX:     dpiX,
			DpiY:     dpiY,
		}
		fillDeviceIdentity(&record)
		records = append(records, record)
		return 1
	})
	procEnumDisplayMonitors.Call(0, 0, callback, 0)
	return records
}

// fillDeviceIdentity reads the adapter entry that owns the GDI name, then the
// monitor entry beneath it. The monitor DeviceID is the interface path, which
// carries the model code and the UID of the connection.
func fillDeviceIdentity(record *displayRecord) {
	adapterName, err := syscall.UTF16PtrFromString(record.GDIName)
	if err != nil {
		return
	}
	var adapter displayDevice
	adapter.Cb = uint32(unsafe.Sizeof(adapter))
	found := false
	for index := uint32(0); index < 32; index++ {
		result, _, _ := procEnumDisplayDevicesW.Call(
			0,
			uintptr(index),
			uintptr(unsafe.Pointer(&adapter)),
			0,
		)
		if result == 0 {
			break
		}
		if utf16ToString(adapter.DeviceName[:]) == record.GDIName {
			found = true
			break
		}
	}
	if found {
		record.AdapterName = utf16ToString(adapter.DeviceString[:])
		record.AdapterDeviceID = utf16ToString(adapter.DeviceID[:])
		record.AdapterKey = utf16ToString(adapter.DeviceKey[:])
	}

	var monitor displayDevice
	monitor.Cb = uint32(unsafe.Sizeof(monitor))
	result, _, _ := procEnumDisplayDevicesW.Call(
		uintptr(unsafe.Pointer(adapterName)),
		0,
		uintptr(unsafe.Pointer(&monitor)),
		uintptr(eddGetDeviceInterface),
	)
	if result == 0 {
		return
	}
	record.MonitorName = utf16ToString(monitor.DeviceString[:])
	record.MonitorDeviceID = utf16ToString(monitor.DeviceID[:])
	record.MonitorKey = utf16ToString(monitor.DeviceKey[:])
}

func reportDisplays(out io.Writer, records []displayRecord) {
	fmt.Fprintf(out, "Displays found: %d\n\n", len(records))
	for index, record := range records {
		primary := "no"
		if record.Primary {
			primary = "yes"
		}
		fmt.Fprintf(out, "[%d] %s\n", index, record.GDIName)
		fmt.Fprintf(out, "    primary:          %s\n", primary)
		fmt.Fprintf(out, "    bounds:           x=%d y=%d w=%d h=%d\n",
			record.Bounds.Left, record.Bounds.Top, record.Bounds.Width(), record.Bounds.Height())
		fmt.Fprintf(out, "    work area:        x=%d y=%d w=%d h=%d\n",
			record.WorkArea.Left, record.WorkArea.Top, record.WorkArea.Width(), record.WorkArea.Height())
		fmt.Fprintf(out, "    dpi:              %dx%d\n", record.DpiX, record.DpiY)
		fmt.Fprintf(out, "    adapter:          %s\n", record.AdapterName)
		fmt.Fprintf(out, "    adapter id:       %s\n", record.AdapterDeviceID)
		fmt.Fprintf(out, "    adapter key:      %s\n", record.AdapterKey)
		fmt.Fprintf(out, "    monitor:          %s\n", record.MonitorName)
		fmt.Fprintf(out, "    monitor id:       %s\n", record.MonitorDeviceID)
		fmt.Fprintf(out, "    monitor key:      %s\n\n", record.MonitorKey)
	}
	fmt.Fprintf(out, "OQ-1: run this before and after a reboot, then compare the\n")
	fmt.Fprintf(out, "monitor id of each display. An id that changes cannot identify a\n")
	fmt.Fprintf(out, "display in a profile. Three displays of the same model must differ.\n")
}
