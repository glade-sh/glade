package gladecli

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

type cancelOnPhaseWriter struct {
	mu     sync.Mutex
	text   strings.Builder
	phase  string
	cancel context.CancelFunc
	once   sync.Once
}

func (w *cancelOnPhaseWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.text.Write(p)
	if strings.Contains(w.text.String(), w.phase) {
		w.once.Do(w.cancel)
	}
	return len(p), nil
}

func TestCheckCancellationBeforeLoadPhases(t *testing.T) {
	for _, phase := range []string{"Loading project", "Loading metadata", "Indexing Apex symbols"} {
		t.Run(phase, func(t *testing.T) {
			root := t.TempDir()
			manifest := `{"packageDirectories":[{"path":"force-app","default":true}]}`
			if phase == "Loading project" {
				manifest = "{"
			}
			writeTestFile(t, filepath.Join(root, "sfdx-project.json"), manifest)
			writeTestFile(t, filepath.Join(root, "force-app/main/default/classes/PhaseControl.cls"), "public class PhaseControl {}")
			if phase == "Loading metadata" {
				writeTestFile(t, filepath.Join(root, "force-app/main/default/objects/PhaseEntry__c/PhaseEntry__c.object-meta.xml"), "<CustomObject>")
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			progress := &cancelOnPhaseWriter{phase: phase, cancel: cancel}
			var output bytes.Buffer
			_, err := runCheck(ctx, []string{"--project", root, "--progress", "--no-cache"}, &output, progress)
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("cancellation at %s = %v; output=%s", phase, err, output.String())
			}
			if strings.Contains(progress.text.String(), "Running semantic checks") {
				t.Fatal("canceled loading continued into semantic analysis")
			}
			if output.Len() != 0 {
				t.Fatalf("canceled check published a result: %s", output.String())
			}
		})
	}
}

func TestTestCancellationBeforeProjectLoad(t *testing.T) {
	for _, perf := range []bool{false, true} {
		name := "normal"
		if perf {
			name = "profiled"
		}
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			writeTestFile(t, filepath.Join(root, "sfdx-project.json"), "{")
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			progress := &cancelOnPhaseWriter{phase: "Discovering tests", cancel: cancel}
			args := []string{"--project", root, "--progress", "--no-serve", "--no-cache", "--test-timeout", "0"}
			if perf {
				args = append(args, "--perf-json", filepath.Join(t.TempDir(), "perf.json"))
			}
			var output bytes.Buffer
			_, err := runTest(ctx, args, &output, progress)
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("canceled startup = %v; output=%s", err, output.String())
			}
		})
	}
}
