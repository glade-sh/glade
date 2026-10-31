//go:build linux

package pluginhost

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

func TestBusyManifestLinkAndDoctorHonorDeadline(t *testing.T) {
	root := t.TempDir()
	exe := writeShellPlugin(t, root, "compat", "#!/bin/sh\nexit 0\n")
	f, err := os.OpenFile(exe, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	store := NewStore(root)
	t.Run("link", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
		defer cancel()
		_, err := store.LinkExecutable(ctx, exe, "link:test")
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("link error = %v, want deadline exceeded while executable is busy", err)
		}
	})
	t.Run("doctor", func(t *testing.T) {
		if err := store.WriteInstalled(InstalledState{Plugins: []InstalledPlugin{{
			Name: "compat", Version: "0.1.0", Executable: exe, Commands: []string{"compat"},
		}}}); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
		defer cancel()
		results, err := store.Doctor(ctx)
		if err != nil || len(results) != 1 || results[0].OK || !strings.Contains(results[0].Message, context.DeadlineExceeded.Error()) {
			t.Fatalf("doctor results = %#v, error = %v; want deadline failure for busy executable", results, err)
		}
	})
}
