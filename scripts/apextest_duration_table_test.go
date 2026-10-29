package scripts

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestCIApexTestDurationTable(t *testing.T) {
	const pkg = "github.com/glade-sh/glade/internal/apextest"
	event := func(second int, action, name string) map[string]any {
		row := map[string]any{"Time": time.Unix(int64(second), 0).UTC().Format(time.RFC3339Nano), "Action": action, "Package": pkg, "Elapsed": 0.01}
		if name != "" {
			row["Test"] = name
		}
		return row
	}
	rows := []map[string]any{
		event(0, "start", ""),
		event(1, "run", "TestSerial"), event(3, "pass", "TestSerial"),
		event(3, "run", "TestParent"), event(4, "run", "TestParent/child"),
		event(8, "pass", "TestParent/child"), event(8, "pass", "TestParent"),
		event(8, "run", "TestQueued"), event(9, "pause", "TestQueued"),
		event(19, "cont", "TestQueued"), event(21, "pass", "TestQueued"),
		event(22, "pass", ""),
	}
	run := func(rows []map[string]any, extra ...string) ([]byte, error) {
		t.Helper()
		var input bytes.Buffer
		for _, row := range rows {
			if err := json.NewEncoder(&input).Encode(row); err != nil {
				t.Fatal(err)
			}
		}
		args := append([]string{"apextest-duration-table.py", "--source", "owned timing fixture"}, extra...)
		cmd := exec.Command("python3", args...)
		cmd.Stdin = &input
		return cmd.CombinedOutput()
	}
	decode := func(data []byte) (string, map[string]int64) {
		t.Helper()
		var table struct {
			Version int
			Package string
			Source  string
			Tests   []struct {
				Name           string
				DurationMillis int64
			}
		}
		if err := json.Unmarshal(data, &table); err != nil || table.Version != 1 || table.Package != pkg {
			t.Fatalf("invalid table: %v\n%s", err, data)
		}
		got := map[string]int64{}
		for _, row := range table.Tests {
			got[row.Name] = row.DurationMillis
		}
		return table.Source, got
	}
	output, err := run(rows)
	if err != nil {
		t.Fatalf("complete log: %v\n%s", err, output)
	}
	_, got := decode(output)
	if len(got) != 3 || got["TestSerial"] != 2000 || got["TestParent"] != 5000 || got["TestQueued"] != 3000 {
		t.Fatalf("wall spans must include children and exclude parent pauses: %v", got)
	}
	for _, tc := range []struct {
		name string
		rows []map[string]any
	}{
		{"incomplete package", rows[:len(rows)-1]},
		{"missing top-level terminal", append(append([]map[string]any{}, rows[:len(rows)-2]...), rows[len(rows)-1])},
		{"duplicate test", append(append([]map[string]any{}, rows[:len(rows)-1]...), event(22, "run", "TestSerial"), event(23, "pass", ""))},
		{"backwards time", []map[string]any{event(0, "start", ""), event(4, "run", "TestA"), event(2, "pass", "TestA"), event(5, "pass", "")}},
		{"resume without pause", []map[string]any{event(0, "start", ""), event(1, "run", "TestA"), event(2, "cont", "TestA"), event(3, "pass", "TestA"), event(4, "pass", "")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if output, err := run(tc.rows); err == nil {
				t.Fatalf("invalid event stream accepted: %s", output)
			}
		})
	}
	output, err = run(rows[:len(rows)-2], "--allow-partial")
	if err != nil {
		t.Fatalf("explicit partial log: %v\n%s", err, output)
	}
	source, got := decode(output)
	if !strings.Contains(source, "PARTIAL") || len(got) != 2 {
		t.Fatalf("partial log must be labelled and omit unfinished tests: %s %v", source, got)
	}
}

func TestCIApexTestDurationMergedMeasurements(t *testing.T) {
	program := `
import importlib.util, json, pathlib, subprocess, sys, tempfile
sys.dont_write_bytecode=True
p='github.com/glade-sh/glade/internal/apextest'
def rows(name,seconds):
 return [dict(Package=p,Action=a,Time='2026-10-24T00:00:%02dZ'%t,**({'Test':n} if n else {})) for t,a,n in [(0,'start',''),(1,'run',name),(seconds+1,'pass',name),(seconds+2,'pass','')]]
with tempfile.TemporaryDirectory() as root:
 paths=[]
 for i,events in enumerate([rows('TestOne',2),rows('TestOne',4)[:-1],rows('TestTwo',3)]):
  path=pathlib.Path(root)/str(i);path.write_text(''.join(json.dumps(e)+'\n' for e in events));paths.extend(['--input',str(path)])
 command=[sys.executable,'apextest-duration-table.py','--source','owned multi-log fixture']+paths
 assert subprocess.run(command,capture_output=True).returncode!=0
 result=json.loads(subprocess.check_output(command+['--allow-partial'],text=True))
 assert {r['name']:r['durationMillis'] for r in result['tests']}=={'TestOne':4000,'TestTwo':3000}
 assert 'PARTIAL' in result['source'] and 'not a matched full-package' in result['source']
`
	if out, err := exec.Command("python3", "-c", program).CombinedOutput(); err != nil {
		t.Fatalf("merged durations: %v\n%s", err, out)
	}
}
