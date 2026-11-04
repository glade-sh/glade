//go:build !linux && !darwin

package testdaemon

import (
	"errors"
	"net"
)

func authenticateDaemonPeer(net.Conn) error {
	return errors.New("daemon peer authentication is unavailable on this platform")
}
