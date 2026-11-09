package apextest

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

// The rows in testdata/conformance/stdlib_surrogate_c14 (captured 2026-11-07
// at API 62.0 and 67.0): Apex strings are UTF-16, so substring, left, right,
// mid and abbreviate can cut inside a surrogate pair and leave a lone
// surrogate that length, getChars, concatenation and the other String members
// then treat as one unit. probe.apex emits one JSON object; each key is a row.
//
// lone_json_roundtrip (JSON.deserialize of text holding a raw lone surrogate)
// keeps the unit, as json_lone_surrogate_c4 captures for every JSON reader.

func TestStdlibSurrogateC14NativeConformance(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "stdlib_surrogate_c14")
	source, err := os.ReadFile(filepath.Join(dir, "probe.apex"))
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 173 {
		t.Fatalf("invalid native observations: %v (%d rows)", err, len(rows))
	}
	observed := map[string]map[string]any{}
	for _, api := range []string{"62.0", "67.0"} {
		program, err := vm.CompileAnonymousWithOptions(string(source), vm.CompileOptions{APIVersion: api})
		if err != nil {
			t.Fatal(err)
		}
		org := storage.NewOrgState()
		machine := vm.New(nil)
		machine.SetOrg(&org)
		var captured string
		machine.SetDebugOutputSink(func(event vm.DebugEvent) {
			if rest, ok := strings.CutPrefix(event.Message, "CAP|n11.surrogate|"); ok {
				captured = rest
			}
		})
		if _, err := machine.Execute(program); err != nil {
			t.Fatalf("api %s: %v", api, err)
		}
		if captured == "" {
			t.Fatalf("api %s: probe emitted no capture", api)
		}
		var got map[string]any
		if err := json.Unmarshal([]byte(captured), &got); err != nil {
			t.Fatalf("api %s: %v: %s", api, err, captured)
		}
		observed[api] = got
	}
	seen := map[string]int{}
	for _, row := range rows[1:] {
		if len(row) != 3 {
			t.Fatalf("invalid row %q", row)
		}
		api, name, observation := row[0], row[1], row[2]
		seen[api]++
		t.Run(api+"/"+name, func(t *testing.T) {
			var want any
			if err := json.Unmarshal([]byte(observation), &want); err != nil {
				t.Fatal(err)
			}
			got, ok := observed[api][name]
			if !ok {
				t.Fatalf("probe emitted no %s", name)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("native=%s; Glade=%v", observation, got)
			}
		})
	}
	if seen["62.0"] != 86 || seen["67.0"] != 86 {
		t.Fatalf("capture shape changed: %v", seen)
	}
}
