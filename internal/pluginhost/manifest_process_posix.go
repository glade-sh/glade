//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package pluginhost

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
	"time"
)

func configureManifestCommand(cmd *exec.Cmd) {
	cmd.WaitDelay = 250 * time.Millisecond
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return os.ErrProcessDone
		}
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if err == nil {
			return nil
		}
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
}
