//go:build linux

package gladecli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/pluginhost"
)

func TestPluginsListBusyManifestHonorsDeadline(t *testing.T) {
	exe := filepath.Join(t.TempDir(), "plugin")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(exe, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var out bytes.Buffer
	err = writePluginsListJSONWithManifestTimeout(context.Background(), &out, []pluginhost.InstalledPlugin{{
		Name: "compat", Version: "0.1.0", Executable: exe, Linked: true, Commands: []string{"compat"},
	}}, 30*time.Millisecond)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("list error = %v, want deadline exceeded while executable is busy", err)
	}
}
