package gladecli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/glade-sh/glade/internal/gladehome"
)

func runToolchain(ctx context.Context, args []string, w io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		printToolchainHelp(w)
		return nil
	}
	switch args[0] {
	case "install":
		return runToolchainInstall(ctx, args[1:], w)
	case "status":
		return runToolchainStatus(w)
	default:
		return fmt.Errorf("unknown toolchain command %q", args[0])
	}
}

func runToolchainInstall(ctx context.Context, args []string, w io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	from := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--from":
			if i+1 >= len(args) {
				return fmt.Errorf("--from requires a path")
			}
			from = args[i+1]
			i++
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}
	var err error
	if from != "" {
		err = gladehome.InstallFrom(from)
	} else {
		err = gladehome.InstallFromCWD()
	}
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "Installed LWC toolchain to %s\n", gladehome.UserShareDir())
	return nil
}

func runToolchainStatus(w io.Writer) error {
	path, ok, detail := gladehome.ToolchainStatus()
	if ok {
		fmt.Fprintf(w, "LWC toolchain: %s (%s)\n", path, detail)
		return nil
	}
	fmt.Fprintf(w, "LWC toolchain: %s (%s)\n", path, detail)
	return fmt.Errorf("toolchain not ready")
}

func printToolchainHelp(w io.Writer) {
	fmt.Fprint(w, strings.TrimSpace(`
Install or inspect the global LWC toolchain used by Lightning Out.

Usage:
  glade toolchain install [--from <glade-checkout>]
  glade toolchain status

The toolchain is installed to ~/.local/share/glade by default. Release installs
and "glade toolchain install" populate this directory so "glade dev vf" works
from any project directory.

Environment:
  GLADE_HOME    Override the glade installation root
  XDG_DATA_HOME Base directory for the global toolchain (default ~/.local/share/glade)
`)+"\n")
}
