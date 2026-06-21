package gladecli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/glade-sh/glade/internal/flagparse"
)

const updateInstallScriptCommand = "curl -fsSL https://glade.sh/install.sh"

func runUpdate(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	parsed, err := flagparse.New("glade update").
		Bool("dry-run", "").
		Parse(args)
	if err != nil {
		return err
	}

	installCommand, err := updateInstallCommand()
	if err != nil {
		return err
	}

	fmt.Fprintf(stdout, "glade %s\n", Version)
	fmt.Fprintln(stdout, "update command:")
	fmt.Fprintf(stdout, "  %s\n", installCommand)
	if parsed.Bool("dry-run") {
		return nil
	}

	if os.Getenv("GLADE_UPDATE_ALLOW_SHELL") != "1" {
		fmt.Fprintln(stdout, "Set GLADE_UPDATE_ALLOW_SHELL=1 to run this command from glade update.")
		return nil
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", installCommand)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

func updateInstallCommand() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", err
	}
	return updateInstallCommandForExecutable(executable), nil
}

func updateInstallCommandForExecutable(executable string) string {
	installDir := filepath.Dir(executable)
	return fmt.Sprintf("%s | env GLADE_INSTALL_DIR=%s sh", updateInstallScriptCommand, shellQuote(installDir))
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
