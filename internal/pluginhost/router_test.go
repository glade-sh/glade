package pluginhost

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestFindByCommandRoot(t *testing.T) {
	state := InstalledState{Version: 1, Plugins: []InstalledPlugin{{
		Name: "compat", Commands: []string{"compat", "surface"},
	}}}

	plugin, ok := FindByCommandRoot(state, "surface")
	if !ok || plugin.Name != "compat" {
		t.Fatalf("expected compat plugin, got %#v ok=%t", plugin, ok)
	}
}

func TestRunPluginStreamsOutputInputAndEnv(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell helper uses sh")
	}
	dir := t.TempDir()
	exe := writeShellPlugin(t, dir, "compat", `#!/bin/sh
printf "plugin stdout: %s\n" "$*"
printf "plugin stderr\n" >&2
read line
printf "stdin: %s\n" "$line"
printf "host=%s api=%s\n" "$GLADE_PLUGIN_HOST" "$GLADE_PLUGIN_API_VERSION"
`)
	var stdout, stderr bytes.Buffer

	code, err := RunPluginWithInput(context.Background(), InstalledPlugin{Executable: exe}, []string{"compat", "x"}, strings.NewReader("from host\n"), &stdout, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	for _, want := range []string{
		"plugin stdout: compat x",
		"stdin: from host",
		"host=glade api=glade.plugin.v1",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout missing %q:\n%s", want, stdout.String())
		}
	}
	if !strings.Contains(stderr.String(), "plugin stderr") {
		t.Fatalf("stderr not streamed: %q", stderr.String())
	}
}

func TestRunPluginReturnsExitCode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell helper uses sh")
	}
	exe := filepath.Join(t.TempDir(), "glade-plugin-compat")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\nexit 23\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	code, err := RunPluginWithInput(context.Background(), InstalledPlugin{Executable: exe}, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if code != 23 {
		t.Fatalf("exit=%d, want 23", code)
	}
}
