package scripts

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestLWCRuntimeNPMTestIncludesBrowserSessionRegression(t *testing.T) {
	data, err := os.ReadFile("../lwcruntime/package.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob("../lwcruntime/test/*.test.mjs")
	if err != nil || len(files) == 0 {
		t.Fatalf("browser suite inventory: %v (%d files)", err, len(files))
	}
	wantFiles := []string{"test/support/browser-session.test.mjs"}
	for _, file := range files {
		wantFiles = append(wantFiles, "test/"+filepath.Base(file))
	}
	slices.Sort(wantFiles)
	for _, tc := range []struct{ route, concurrency, want string }{
		{"default", "", "2"},
		{"sequential", "1", "1"},
		{"explicit-concurrency", "3", "3"},
	} {
		t.Run(tc.route, func(t *testing.T) {
			root := t.TempDir()
			work := filepath.Join(root, "lwcruntime")
			bin := filepath.Join(root, "bin")
			if err := os.MkdirAll(bin, 0o700); err != nil {
				t.Fatal(err)
			}
			for _, file := range wantFiles {
				path := filepath.Join(work, file)
				if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, nil, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			// Run the actual npm script, replacing expensive builds and Node with
			// isolated commands that expose shell-expanded test arguments.
			for name, script := range map[string]string{
				"npm":  "#!/bin/sh\nexit 0\n",
				"go":   "#!/bin/sh\nexit 0\n",
				"node": "#!/bin/sh\nprintf '%s\\n' \"$@\" >\"$NODE_ARGS_FILE\"\n",
			} {
				if err := os.WriteFile(filepath.Join(bin, name), []byte(script), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			argsPath := filepath.Join(root, "node-args")
			cmd := exec.Command("sh", "-c", manifest.Scripts["test"])
			cmd.Dir = work
			cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
				"NODE_ARGS_FILE="+argsPath, "GLADE_LWCRUNTIME_TEST_CONCURRENCY="+tc.concurrency)
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("npm test script: %v\n%s", err, output)
			}
			data, err := os.ReadFile(argsPath)
			if err != nil {
				t.Fatal(err)
			}
			args := strings.Split(strings.TrimSpace(string(data)), "\n")
			wantFlags := []string{"--test", "--test-concurrency=" + tc.want}
			if len(args) < len(wantFlags) || !reflect.DeepEqual(args[:len(wantFlags)], wantFlags) {
				t.Fatalf("Node flags = %v, want %v", args, wantFlags)
			}
			gotFiles := args[len(wantFlags):]
			slices.Sort(gotFiles)
			if !reflect.DeepEqual(gotFiles, wantFiles) {
				t.Errorf("suite selects %d files, want %d; missing %s", len(gotFiles), len(wantFiles), "test/support/browser-session.test.mjs")
				t.Logf("got %v\nwant %v", gotFiles, wantFiles)
			}
		})
	}
}
