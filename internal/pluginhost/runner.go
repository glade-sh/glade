package pluginhost

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

func RunPlugin(ctx context.Context, plugin InstalledPlugin, args []string, stdout, stderr io.Writer) (int, error) {
	return RunPluginWithInput(ctx, plugin, args, os.Stdin, stdout, stderr)
}

func RunPluginWithInput(ctx context.Context, plugin InstalledPlugin, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	env := append(pluginSubprocessEnv(os.Environ()),
		"GLADE_PLUGIN_HOST=glade",
		"GLADE_PLUGIN_API_VERSION="+APIVersion,
	)
	// Linux can briefly reject exec while another process holds the executable
	// open for writing. Retry only a failed start; a plugin that ran is never rerun.
	const attempts = 5
	var err error
	for attempt := 0; ; attempt++ {
		cmd := exec.CommandContext(ctx, plugin.Executable, args...)
		cmd.Stdin = stdin
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		cmd.Env = env
		err = cmd.Run()
		if !errors.Is(err, syscall.ETXTBSY) || attempt == attempts-1 {
			break
		}
		timer := time.NewTimer(25 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return 1, ctx.Err()
		case <-timer.C:
		}
	}
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
