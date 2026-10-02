package tui

import "syscall"

const getTermios, setTermios = syscall.TIOCGETA, syscall.TIOCSETA
