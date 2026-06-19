package gladecli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/glade-sh/glade/internal/cliui"
	"github.com/glade-sh/glade/internal/gladehome"
)

func runToolchain(ctx context.Context, args []string, w io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		_ = cliui.WriteCommandHelp(w, []string{"toolchain"})
		return nil
	}
	switch args[0] {
	case "install":
		return runToolchainInstall(ctx, args[1:], w)
	case "status":
		return runToolchainStatus(args[1:], w)
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

type toolchainStatusJSON struct {
	OK     bool   `json:"ok"`
	Path   string `json:"path"`
	Detail string `json:"detail"`
}

func runToolchainStatus(args []string, w io.Writer) error {
	jsonOut := false
	for _, arg := range args {
		switch arg {
		case "--json", "-j":
			jsonOut = true
		default:
			return fmt.Errorf("unknown toolchain status argument %q", arg)
		}
	}
	path, ok, detail := gladehome.ToolchainStatus()
	if jsonOut {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		if err := enc.Encode(toolchainStatusJSON{OK: ok, Path: path, Detail: detail}); err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("toolchain not ready")
		}
		return nil
	}
	if ok {
		fmt.Fprintf(w, "LWC toolchain: %s (%s)\n", path, detail)
		return nil
	}
	fmt.Fprintf(w, "LWC toolchain: %s (%s)\n", path, detail)
	return fmt.Errorf("toolchain not ready")
}
