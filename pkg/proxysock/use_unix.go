//go:build linux || darwin || freebsd || openbsd || netbsd
// +build linux darwin freebsd openbsd netbsd

package proxysock

import "syscall"

const (
	sigUSR1 = syscall.SIGUSR1
	sigUSR2 = syscall.SIGUSR2
)
