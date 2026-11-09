package scripts

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCIShardChunksPreserveEvents(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(fmt.Sprintf("failure=%v", fail), func(t *testing.T) {
			dir := t.TempDir()
			bin := filepath.Join(dir, "bin")
			os.Mkdir(bin, 0700)
			names := make([]string, 1800)
			for i := range names {
				names[i] = fmt.Sprintf("TestSynthetic%04d_%s", i, strings.Repeat("A", 65))
			}
			other := "TestOther"
			discovery := strings.Join(append(append([]string{}, names...), other), "\n") + "\n"
			regex := "^(?:" + strings.Join(names, "|") + ")$"
			plan := map[string]any{"version": 1, "package": fixturePackage, "historyUsed": false, "shards": []any{
				map[string]any{"index": 0, "tests": names, "estimatedDurationMillis": 0, "regex": regex},
				map[string]any{"index": 1, "tests": []string{other}, "estimatedDurationMillis": 0, "regex": "^(?:TestOther)$"},
			}}
			data, _ := json.Marshal(plan)
			os.WriteFile(filepath.Join(dir, "plan"), data, 0600)
			os.WriteFile(filepath.Join(dir, "discovery"), []byte(discovery), 0600)
			writeApexFixtureExecutable(t, filepath.Join(bin, "planner"), "#!/bin/sh\ncat \"$FIXTURE_PLAN\"\n")
			writeApexFixtureExecutable(t, filepath.Join(bin, "renderer"), "#!/bin/sh\ncat >/dev/null\n")
			writeApexFixtureExecutable(t, filepath.Join(bin, "go"), `#!/usr/bin/env python3
import json, os, sys
if '-list' in sys.argv:
 print(open(os.environ['FIXTURE_DISCOVERY']).read(), end=''); sys.exit(0)
args = sys.argv[1:]
with open(os.environ['FIXTURE_CALLS'], 'a') as f: f.write(str(max(len(x.encode()) for x in args))+'\n')
r = args[args.index('-run')+1]
names = r[4:-2].split('|')
for name in names:
 print(json.dumps({'Action':'run','Package':os.environ['FIXTURE_PACKAGE'],'Test':name}))
 print(json.dumps({'Action':'pass','Package':os.environ['FIXTURE_PACKAGE'],'Test':name,'Elapsed':0}))
print(json.dumps({'Action':'pass','Package':os.environ['FIXTURE_PACKAGE'],'Elapsed':0}))
sys.exit(7 if os.environ['FIXTURE_FAIL']=='true' else 0)
`)
			artifacts := filepath.Join(dir, "artifacts")
			calls := filepath.Join(dir, "calls")
			cmd := exec.Command("bash", "ci-go-test.sh", "apex-shard", "0")
			cmd.Env = append(os.Environ(), "PATH="+bin+":"+os.Getenv("PATH"), "CI_SHARD_PLANNER="+filepath.Join(bin, "planner"), "CI_TESTLOG_RENDERER="+filepath.Join(bin, "renderer"), "CI_APEXTEST_ARTIFACT_DIR="+artifacts, "FIXTURE_PLAN="+filepath.Join(dir, "plan"), "FIXTURE_DISCOVERY="+filepath.Join(dir, "discovery"), "FIXTURE_CALLS="+calls, "FIXTURE_PACKAGE="+fixturePackage, fmt.Sprintf("FIXTURE_FAIL=%v", fail))
			out, err := cmd.CombinedOutput()
			if (err != nil) != fail {
				t.Fatalf("unexpected result: %v\n%s", err, out)
			}
			b, err := os.ReadFile(calls)
			if err != nil {
				t.Fatal(err)
			}
			lines := strings.Fields(string(b))
			if len(lines) < 2 {
				t.Fatalf("expected chunks, calls=%s", b)
			}
			for _, line := range lines {
				var n int
				fmt.Sscan(line, &n)
				if n > 100000 {
					t.Fatalf("argument too long: %d", n)
				}
			}
			events, err := os.ReadFile(filepath.Join(artifacts, "events.json"))
			if err != nil {
				t.Fatal(err)
			}
			if strings.Count(string(events), `"Action": "run"`) != len(names) {
				t.Fatalf("combined events missing tests")
			}
		})
	}
}
