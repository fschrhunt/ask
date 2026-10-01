package status

import (
	"os"
	"syscall"
	"unsafe"
)

// terminalWidth uses the stdout terminal width, falling back to the pipe contract.
func terminalWidth() int {
	var size [4]uint16
	_, _, e := syscall.Syscall(syscall.SYS_IOCTL, os.Stdout.Fd(), syscall.TIOCGWINSZ, uintptr(unsafe.Pointer(&size)))
	if e == 0 && size[1] > 0 {
		return int(size[1])
	}
	return 120
}
