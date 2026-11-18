//go:build linux

package pluginhost

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestRunPluginRetriesWhileExecutableBusy(t *testing.T) {
	exe := writeShellPlugin(t, t.TempDir(), "compat", "#!/bin/sh\nprintf ran\n")
	f, err := os.OpenFile(exe, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	done := make(chan struct{})
	go func() { time.Sleep(30 * time.Millisecond); f.Close(); close(done) }()
	defer func() { <-done }()

	var stdout, stderr bytes.Buffer
	code, err := RunPluginWithInput(context.Background(), InstalledPlugin{Name: "compat", Executable: exe}, nil, strings.NewReader(""), &stdout, &stderr)
	if err != nil || code != 0 || stdout.String() != "ran" {
		t.Fatalf("code=%d err=%v stdout=%q; want success after writer closes", code, err, stdout.String())
	}
}
