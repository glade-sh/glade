package gladecli

import (
	"context"
	_ "embed"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/glade-sh/glade/internal/flagparse"
)

//go:embed update_installer.sh
var updateInstaller string

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

	executable, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, "sh", "-c", updateInstaller)
	cmd.Env = append(os.Environ(), "GLADE_INSTALL_DIR="+filepath.Dir(executable))
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
	return fmt.Sprintf("env GLADE_UPDATE_ALLOW_SHELL=1 %s update", shellQuote(executable))
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
