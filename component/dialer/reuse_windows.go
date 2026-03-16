package dialer

import (
	"errors"
	"net"
	"syscall"

	"golang.org/x/sys/windows"
)

func addrReuseToListenConfig(lc *net.ListenConfig) {
	chain := lc.Control

	lc.Control = func(network, address string, c syscall.RawConn) (err error) {
		defer func() {
			if err == nil && chain != nil {
				err = chain(network, address, c)
			}
		}()

		return c.Control(func(fd uintptr) {
			windows.SetsockoptInt(windows.Handle(fd), windows.SOL_SOCKET, windows.SO_REUSEADDR, 1)
		})
	}
}

func enableTFO(fd uintptr) error {
	// 在非 Linux 系统下什么都不做，或者实现对应平台的逻辑
	return nil
}

func kernelCheck(fd uintptr) (int, error) {
	return -1, errors.New("kernelCheck not implemented")
}
