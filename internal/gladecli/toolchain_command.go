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
	if len(args) > 0 && args[0] == "dataweave" {
		return runDataWeaveToolchainInstall(ctx, args[1:], w)
	}
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
	if len(args) > 0 && args[0] == "dataweave" {
		return runDataWeaveToolchainStatus(args[1:], w)
	}
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

func runDataWeaveToolchainInstall(ctx context.Context, args []string, w io.Writer) error {
	javaHome := ""
	jsonOut := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--java-home":
			if i+1 >= len(args) {
				return fmt.Errorf("--java-home requires a Java17 JDK path")
			}
			javaHome = args[i+1]
			i++
		case "--json", "-j":
			jsonOut = true
		case "--help", "-h":
			_, err := fmt.Fprintln(w, "Usage: glade toolchain install dataweave --java-home <Java17-JDK> [--json]")
			return err
		default:
			return fmt.Errorf("unknown DataWeave install argument %q", args[i])
		}
	}
	status, err := gladehome.InstallDataWeave(ctx, javaHome)
	if err != nil {
		return err
	}
	if jsonOut {
		return json.NewEncoder(w).Encode(status)
	}
	_, err = fmt.Fprintf(w, "Installed DataWeave %s toolchain to %s\n", status.EngineVersion, status.Path)
	return err
}
func runDataWeaveToolchainStatus(args []string, w io.Writer) error {
	jsonOut := false
	for _, arg := range args {
		switch arg {
		case "--json", "-j":
			jsonOut = true
		case "--help", "-h":
			_, err := fmt.Fprintln(w, "Usage: glade toolchain status dataweave [--json]")
			return err
		default:
			return fmt.Errorf("unknown DataWeave status argument %q", arg)
		}
	}
	status := gladehome.DataWeaveStatus()
	if jsonOut {
		if err := json.NewEncoder(w).Encode(status); err != nil {
			return err
		}
	} else {
		if _, err := fmt.Fprintf(w, "DataWeave toolchain: %s (%s)\n", status.Path, status.Detail); err != nil {
			return err
		}
	}
	if !status.OK {
		return fmt.Errorf("DataWeave toolchain not ready")
	}
	return nil
}
