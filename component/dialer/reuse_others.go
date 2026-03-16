//go:build !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris && !windows
// +build !darwin,!dragonfly,!freebsd,!linux,!netbsd,!openbsd,!solaris,!windows

package dialer

import (
	"net"
)

func addrReuseToListenConfig(*net.ListenConfig) {}

func enableTFO(fd uintptr) error {
	// 在非 Linux 系统下什么都不做，或者实现对应平台的逻辑
	return nil
}

func kernelCheck(fd uintptr) (int, error) {
	return -1, errors.New("kernelCheck not implemented")
}
