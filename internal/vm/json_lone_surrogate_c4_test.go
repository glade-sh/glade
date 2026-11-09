package vm

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

// The rows in ../apextest/testdata/conformance/json_lone_surrogate_c4
// (captured 2026-11-07, anonymous Apex at API 62.0 and 67.0): probe.apex feeds
// escaped (\uD800, \uD83D, \uDC00, pairs, mixed) and raw lone surrogates to
// JSON.deserialize (String and List<String>), JSON.deserializeUntyped and
// JSONParser.getText, and re-serializes the result; literal.apex reads the
// Apex literal '\uD800'. Native keeps every lone unit and joins a high and a
// low unit into one code point whether each was escaped or raw.
func TestJSONLoneSurrogateC4OrgRows(t *testing.T) {
	dir := "../apextest/testdata/conformance/json_lone_surrogate_c4/"
	file, err := os.Open(dir + "org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 29 {
		t.Fatalf("rows=%d err=%v", len(rows), err)
	}
	observed := map[string]map[string]any{}
	for _, api := range []string{"62.0", "67.0"} {
		observed[api] = map[string]any{}
		for _, probe := range []struct{ file, prefix string }{{"probe.apex", "CAP|json|"}, {"literal.apex", "CAP|json_lit|"}} {
			source, err := os.ReadFile(dir + probe.file)
			if err != nil {
				t.Fatal(err)
			}
			program, err := CompileAnonymousWithOptions(string(source), CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatalf("api %s %s: %v", api, probe.file, err)
			}
			machine := New(nil)
			var captured string
			machine.SetDebugOutputSink(func(event DebugEvent) {
				if rest, ok := strings.CutPrefix(event.Message, probe.prefix); ok {
					captured = rest
				}
			})
			if _, err := machine.Execute(program); err != nil {
				t.Fatalf("api %s %s: %v", api, probe.file, err)
			}
			var got any
			if err := json.Unmarshal([]byte(captured), &got); err != nil {
				t.Fatalf("api %s %s: %v: %q", api, probe.file, err, captured)
			}
			if probe.file == "literal.apex" {
				observed[api]["literal_escape"] = got
				continue
			}
			for key, value := range got.(map[string]any) {
				observed[api][key] = value
			}
		}
	}
	for _, row := range rows[1:] {
		api, name, observation := row[0], row[1], row[2]
		t.Run(api+"/"+name, func(t *testing.T) {
			var want any
			if err := json.Unmarshal([]byte(observation), &want); err != nil {
				t.Fatal(err)
			}
			if got := observed[api][name]; !reflect.DeepEqual(got, want) {
				t.Fatalf("native %v, Glade %v", want, got)
			}
		})
	}
}
