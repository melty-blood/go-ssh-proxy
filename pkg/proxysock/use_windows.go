//go:build windows
// +build windows

package proxysock

import "syscall"

// type Signal int

const (
	sigUSR1 = syscall.Signal(0xa)
	sigUSR2 = syscall.Signal(0xc)
)
