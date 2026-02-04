//go:build !windows
// +build !windows

package src

// enableVirtualTerminal 在非 Windows 系统上不需要执行操作
func enableVirtualTerminal() {
	// Linux/macOS 通常原生支持 ANSI 转义序列，无需特殊设置
}
