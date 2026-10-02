package status

import (
	"os"
	"syscall"
	"unsafe"
)

// terminalSize reads terminal dimensions from the selected stream.
func terminalSize(file *os.File) (int, int, bool) {
	var size [4]uint16
	_, _, err := syscall.Syscall(syscall.SYS_IOCTL, file.Fd(), syscall.TIOCGWINSZ, uintptr(unsafe.Pointer(&size)))
	if err != 0 {
		return 0, 0, false
	}
	width, height := int(size[1]), int(size[0])
	if width == 0 {
		width = 80
	}
	if height == 0 {
		height = 24
	}
	return width, height, true
}

// terminalWidth uses the stdout terminal width, falling back to the pipe contract.
func terminalWidth() int {
	var size [4]uint16
	_, _, e := syscall.Syscall(syscall.SYS_IOCTL, os.Stdout.Fd(), syscall.TIOCGWINSZ, uintptr(unsafe.Pointer(&size)))
	if e == 0 && size[1] > 0 {
		return int(size[1])
	}
	return 120
}
