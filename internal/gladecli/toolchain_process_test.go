package gladecli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/glade-sh/glade/internal/testutil/toolchaininstall"
)

func TestToolchainInstallProjectTwoProcessesColdShare(t *testing.T) {
	toolchaininstall.ColdShare(t, []string{"install_from", "install_cwd"}, func(route string) error {
		args := []string{"toolchain", "install"}
		if route == "install_from" {
			src, err := os.Getwd()
			if err != nil {
				return err
			}
			args = append(args, "--from", src)
		}
		var stdout, stderr bytes.Buffer
		if code := Run(context.Background(), args, &stdout, &stderr); code != 0 {
			return fmt.Errorf("glade toolchain install exited %d: %s%s", code, &stdout, &stderr)
		}
		return nil
	})
}
