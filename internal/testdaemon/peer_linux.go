package testdaemon

import (
	"fmt"
	"net"
	"os"

	"golang.org/x/sys/unix"
)

func authenticateDaemonPeer(conn net.Conn) error {
	c, ok := conn.(*net.UnixConn)
	if !ok {
		return fmt.Errorf("not a Unix connection")
	}
	raw, err := c.SyscallConn()
	if err != nil {
		return err
	}
	var uid uint32
	var inner error
	if err := raw.Control(func(fd uintptr) {
		var cred *unix.Ucred
		cred, inner = unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED)
		if inner == nil {
			uid = cred.Uid
		}
	}); err != nil {
		return err
	}
	if inner != nil {
		return inner
	}
	if uid != uint32(os.Geteuid()) {
		return fmt.Errorf("unexpected peer owner")
	}
	return nil
}
