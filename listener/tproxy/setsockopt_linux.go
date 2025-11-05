//go:build linux
// +build linux

package tproxy

import (
	"net"
	"reflect"
	"syscall"
)

// isUDPConn 判断传入的RawConn是否为UDP连接
func israwConn(rc syscall.RawConn) bool {
	// 通过类型检查判断是否为UDP连接
	// 在实际应用中，可能需要根据不同平台调整判断方式
	// 这里使用反射尝试获取原始连接类型信息
	connType := reflect.TypeOf(rc)
	//fmt.Println("isUDPConn connType:", connType.String())
	return connType.String() == "*net.rawConn"
}

func setsockopt(rc syscall.RawConn, addr string) error {
	isIPv6 := true
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}
	ip := net.ParseIP(host)

	if ip != nil && ip.To4() != nil {
		isIPv6 = false
	}
	//fmt.Println("setsockopt ip:", ip, isIPv6)

	rc.Control(func(fd uintptr) {
		err = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
		//fmt.Println("SetsockoptInt int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1", err)
		if err == nil {
			err = syscall.SetsockoptInt(int(fd), syscall.SOL_IP, syscall.IP_TRANSPARENT, 1)
		}
		//fmt.Println("SetsockoptInt int(fd), syscall.SOL_IP, syscall.IP_TRANSPARENT, 1", err)
		if err == nil && isIPv6 {
			err = syscall.SetsockoptInt(int(fd), syscall.SOL_IPV6, IPV6_TRANSPARENT, 1)
			//fmt.Println("SetsockoptInt int(fd), syscall.SOL_IPV6, IPV6_TRANSPARENT, 1", err)
		}

		if israwConn(rc) {
			if err == nil {
				err = syscall.SetsockoptInt(int(fd), syscall.SOL_IP, syscall.IP_RECVORIGDSTADDR, 1)
				//fmt.Println("SetsockoptInt int(fd), syscall.SOL_IP, syscall.IP_RECVORIGDSTADDR, 1", err)
			}
			if err == nil && isIPv6 {
				err = syscall.SetsockoptInt(int(fd), syscall.SOL_IPV6, IPV6_RECVORIGDSTADDR, 1)
				//fmt.Println("SetsockoptInt int(fd), syscall.SOL_IPV6, IPV6_RECVORIGDSTADDR, 1", err)
			}
		}
	})

	return err
}
