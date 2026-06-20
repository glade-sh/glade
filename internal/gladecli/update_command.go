package gladecli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/glade-sh/glade/internal/flagparse"
)

const updateInstallCommand = "curl -fsSL https://glade.sh/install.sh | sh"

func runUpdate(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	parsed, err := flagparse.New("glade update").
		Bool("dry-run", "").
		Parse(args)
	if err != nil {
		return err
	}

	fmt.Fprintf(stdout, "glade %s\n", Version)
	fmt.Fprintln(stdout, "update command:")
	fmt.Fprintf(stdout, "  %s\n", updateInstallCommand)
	if parsed.Bool("dry-run") {
		return nil
	}

	if os.Getenv("GLADE_UPDATE_ALLOW_SHELL") != "1" {
		fmt.Fprintln(stdout, "Set GLADE_UPDATE_ALLOW_SHELL=1 to run this command from glade update.")
		return nil
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", updateInstallCommand)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}
