package pluginhost

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
)

func RunPlugin(ctx context.Context, plugin InstalledPlugin, args []string, stdout, stderr io.Writer) (int, error) {
	return RunPluginWithInput(ctx, plugin, args, os.Stdin, stdout, stderr)
}

func RunPluginWithInput(ctx context.Context, plugin InstalledPlugin, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	cmd := exec.CommandContext(ctx, plugin.Executable, args...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Env = append(pluginSubprocessEnv(os.Environ()),
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

func pluginSubprocessEnv(environ []string) []string {
	allowedNames := map[string]bool{
		"HOME":       true,
		"PATH":       true,
		"TMPDIR":     true,
		"TEMP":       true,
		"TMP":        true,
		"SystemRoot": true,
		"WINDIR":     true,
		"COMSPEC":    true,
		"PATHEXT":    true,
	}
	out := make([]string, 0, len(environ))
	for _, entry := range environ {
		name, _, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		if allowedNames[name] || strings.HasPrefix(name, "GLADE_") {
			out = append(out, entry)
		}
	}
	return out
}
