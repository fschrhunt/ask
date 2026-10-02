package tui

import "syscall"

const getTermios, setTermios = syscall.TCGETS, syscall.TCSETS
