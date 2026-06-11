package pluginhost

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
)

func RunPlugin(ctx context.Context, plugin InstalledPlugin, args []string, stdout, stderr io.Writer) (int, error) {
	return RunPluginWithInput(ctx, plugin, args, os.Stdin, stdout, stderr)
}

func RunPluginWithInput(ctx context.Context, plugin InstalledPlugin, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	cmd := exec.CommandContext(ctx, plugin.Executable, args...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Env = append(os.Environ(),
		"GLADE_PLUGIN_HOST=glade",
		"GLADE_PLUGIN_API_VERSION="+APIVersion,
	)
	err := cmd.Run()
	if err == nil {
		return 0, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), nil
	}
	return 1, err
}
