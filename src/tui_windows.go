//go:build windows
// +build windows

package src

import "golang.org/x/sys/windows"

// enableVirtualTerminal 在 Windows 上启用虚拟终端处理
func enableVirtualTerminal() {
	handle, err := windows.GetStdHandle(windows.STD_OUTPUT_HANDLE)
	if err != nil {
		return
	}
	var mode uint32
	if err := windows.GetConsoleMode(handle, &mode); err != nil {
		return
	}
	// 启用虚拟终端处理
	mode |= windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING
	windows.SetConsoleMode(handle, mode)
}
