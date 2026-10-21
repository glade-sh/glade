package gladecli

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
)

func TestDataWeaveToolchainUnsupported(t *testing.T) {
	home := t.TempDir()
	t.Setenv("GLADE_HOME", home)
	t.Setenv("GLADE_ROOT", "")
	for _, args := range [][]string{
		{"install", "dataweave"},
		{"install", "dataweave", "--java-home", home, "--json"},
		{"install", "dataweave", "--help"},
		{"status", "dataweave"},
		{"status", "dataweave", "--json"},
		{"status", "dataweave", "--help"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var output bytes.Buffer
			err := runToolchain(context.Background(), args, &output)
			if err == nil || !strings.Contains(err.Error(), "DataWeave is unsupported") {
				t.Errorf("error = %v, want unsupported DataWeave", err)
			}
			if output.Len() != 0 {
				t.Errorf("unsupported command produced toolchain output: %s", output.String())
			}
		})
	}
	entries, err := os.ReadDir(home)
	if err != nil || len(entries) != 0 {
		t.Fatalf("unsupported toolchain modified installation root: entries=%v err=%v", entries, err)
	}
}

func TestToolchainHelpExcludesDataWeaveInstallation(t *testing.T) {
	var output bytes.Buffer
	if err := runToolchain(context.Background(), []string{"--help"}, &output); err != nil {
		t.Fatal(err)
	}
	for _, removed := range []string{"install dataweave", "status dataweave", "--java-home"} {
		if strings.Contains(output.String(), removed) {
			t.Errorf("toolchain help advertises %q", removed)
		}
	}
	if !strings.Contains(output.String(), "DataWeave is unsupported") {
		t.Error("toolchain help does not explain the unsupported component")
	}
}
