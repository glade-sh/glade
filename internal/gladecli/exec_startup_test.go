package gladecli

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"sync"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/vm"
)

func TestRunExecPreservesCallerGCPercent(t *testing.T) {
	t.Setenv("GOGC", "")
	old := debug.SetGCPercent(73)
	defer debug.SetGCPercent(old)
	for _, test := range []struct {
		args      []string
		wantError bool
	}{
		{wantError: true},
		{args: []string{"--json", "System.assert(true);"}},
	} {
		if err := runExec(context.Background(), test.args, io.Discard); (err != nil) != test.wantError {
			t.Fatalf("runExec(%q) error = %v, wantError = %v", test.args, err, test.wantError)
		}
		if got := debug.SetGCPercent(73); got != 73 {
			t.Fatalf("runExec(%q) changed caller GC percent to %d", test.args, got)
		}
	}
}

func TestRunExecProjectLoadError(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), "{")
	_, _, want := loadProjectIndex(root)
	if want == nil {
		t.Fatal("invalid project unexpectedly loaded")
	}
	err := runExec(context.Background(), []string{"--project", root, "System.assert(false);"}, io.Discard)
	if err == nil || err.Error() != want.Error() {
		t.Fatalf("exec error = %v, want project load error %v", err, want)
	}
}

func TestRunExecCanceledBeforeProjectLoad(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := runExec(ctx, []string{"--project", filepath.Join(t.TempDir(), "missing"), "System.assert(false);"}, io.Discard)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("exec error = %v, want context.Canceled", err)
	}
}

func TestRunExecCanceledDuringProjectLoadJoinsAndPreservesErrors(t *testing.T) {
	// Warm startup first so the caller can reach the join while the loader is
	// blocked, including when the race detector slows its first initialization.
	vm.PrewarmPlatformIndexes()
	projectErr := errors.New("captured project load error")
	orgErr := errors.New("captured org load error")
	for _, test := range []struct {
		name       string
		projectErr error
		orgErr     error
		source     string
		apiVersion string
		dbError    bool
		wantError  bool
	}{
		{name: "project", projectErr: projectErr, wantError: true},
		{name: "org", orgErr: orgErr, wantError: true},
		{name: "project before org", projectErr: projectErr, orgErr: orgErr, wantError: true},
		{name: "successful load"},
		{name: "DB error after successful load", dbError: true, wantError: true},
		{name: "source version error after successful load", apiVersion: "68.0", wantError: true},
		{name: "anonymous compilation error after successful load", source: "Integer value = ;", wantError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
			loadOrg := !test.dbError
			load := loadExecProject(root, loadOrg)
			if load.err != nil || load.orgErr != nil {
				t.Fatalf("fixture load errors: project=%v org=%v", load.err, load.orgErr)
			}
			load.err, load.orgErr = test.projectErr, test.orgErr
			if test.apiVersion != "" {
				load.index.Project.SourceAPIVersion = test.apiVersion
			}
			source := test.source
			if source == "" {
				source = "System.assert(true);"
			}
			args := []string{"--project", root, "--json"}
			if test.dbError {
				dbPath := filepath.Join(root, "not-a-db")
				if err := os.Mkdir(dbPath, 0o755); err != nil {
					t.Fatal(err)
				}
				args = append(args, "--db", dbPath)
			}
			args = append(args, source)
			// Without mid-load cancellation this follows canonical's ordinary
			// DB/source/compilation path. Cancellation must leave that result
			// unchanged, including success and the exact diagnostic text.
			want := runExecWithProjectLoader(context.Background(), args, io.Discard,
				func(string, bool) execProjectLoad { return load })
			if (want != nil) != test.wantError || errors.Is(want, context.Canceled) {
				t.Fatalf("uncancelled control error = %v, wantError = %v", want, test.wantError)
			}
			if test.projectErr != nil && want != test.projectErr {
				t.Fatalf("uncancelled control error = %v, want project error %v", want, test.projectErr)
			}
			if test.projectErr == nil && test.orgErr != nil && want != test.orgErr {
				t.Fatalf("uncancelled control error = %v, want org error %v", want, test.orgErr)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			started := make(chan struct{})
			release := make(chan struct{})
			finished := make(chan struct{})
			var releaseOnce sync.Once
			unblock := func() { releaseOnce.Do(func() { close(release) }) }
			defer unblock()
			result := make(chan error, 1)
			go func() {
				result <- runExecWithProjectLoader(ctx, args, io.Discard,
					func(gotRoot string, gotLoadOrg bool) execProjectLoad {
						defer close(finished)
						if gotRoot != root || gotLoadOrg != loadOrg {
							t.Errorf("loader arguments = (%q, %v), want (%q, %v)", gotRoot, gotLoadOrg, root, loadOrg)
						}
						close(started)
						<-release
						return load
					})
			}()
			select {
			case <-started:
			case <-time.After(10 * time.Second):
				t.Fatal("concurrent project loader did not start")
			}
			cancel()
			select {
			case err := <-result:
				unblock()
				<-finished
				t.Fatalf("exec returned before joining the blocked loader: %v", err)
			case <-time.After(100 * time.Millisecond):
			}
			unblock()
			select {
			case err := <-result:
				if (err == nil) != (want == nil) || (err != nil && err.Error() != want.Error()) {
					t.Fatalf("cancelled exec error = %v, want uncancelled control error %v", err, want)
				}
				select {
				case <-finished:
				default:
					t.Fatal("exec returned before loader finished")
				}
			case <-time.After(10 * time.Second):
				t.Fatal("exec did not return after loader completed")
			}
		})
	}
}
