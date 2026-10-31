package gladecli

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/startupcache"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

// A fresh pre-frame-formula DAP snapshot must be rebuilt, then the rebuilt
// snapshot must remain usable on a second (warm) load.
func TestFlowFormulaFrameRejectLegacyDAPCache(t *testing.T) {
	const fixture = "../apextest/testdata/conformance/flow_formula_frame"
	const legacyVersion = 12
	t.Setenv("GLADE_HOME", t.TempDir())
	rows, err := os.ReadFile(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(fixture, "project")
			err := filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
				if walkErr != nil || entry.IsDir() {
					return walkErr
				}
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				rel, err := filepath.Rel(source, path)
				if err != nil {
					return err
				}
				writeTestFile(t, filepath.Join(root, rel), strings.ReplaceAll(string(data), "62.0", api))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			// The DAP loader builds only source-declared schema. Supply minimal
			// synthetic standard-object metadata so it can attach the owned flows.
			for object, fields := range map[string][]string{
				"Account": {"Name", "Site", "Description", "AccountNumber", "TickerSymbol", "BillingCity"},
				"Contact": {"LastName", "Description", "Title", "Department"},
			} {
				base := filepath.Join(root, "force-app", "main", "default", "objects", object)
				writeTestFile(t, filepath.Join(base, object+".object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>`+object+`</label><pluralLabel>`+object+`s</pluralLabel></CustomObject>`)
				for _, field := range fields {
					writeTestFile(t, filepath.Join(base, "fields", field+".field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>`+field+`</fullName><label>`+field+`</label><type>Text</type><length>255</length></CustomField>`)
				}
			}
			if _, _, _, err := loadDAPStartupState(root); err != nil {
				t.Fatal(err)
			}
			entry, err := startupcache.Read(root, startupcache.SubdirDAP)
			if err != nil || entry == nil {
				t.Fatalf("initial cache: entry=%v err=%v", entry != nil, err)
			}
			encoded, err := json.Marshal(entry.Org)
			if err != nil {
				t.Fatal(err)
			}
			var raw any
			if err := json.Unmarshal(encoded, &raw); err != nil {
				t.Fatal(err)
			}
			removed := map[string]int{}
			var strip func(any)
			strip = func(value any) {
				switch value := value.(type) {
				case map[string]any:
					for key, child := range value {
						if key == "flowFrameFormula" || key == "formulas" {
							removed[key]++
							delete(value, key)
							continue
						}
						strip(child)
					}
				case []any:
					for _, child := range value {
						strip(child)
					}
				}
			}
			strip(raw)
			if removed["flowFrameFormula"] == 0 || removed["formulas"] == 0 {
				t.Fatalf("old fixture lacks captured fields: %#v", removed)
			}
			encoded, err = json.Marshal(raw)
			if err != nil {
				t.Fatal(err)
			}
			var staleOrg storage.OrgState
			if err := json.Unmarshal(encoded, &staleOrg); err != nil {
				t.Fatal(err)
			}
			entry.Org = staleOrg
			entry.Version, entry.BuiltAt = legacyVersion, "stale-flow-formula-model"
			if !startupcache.Fresh(entry, root, legacyVersion) {
				t.Fatal("legacy fixture is not otherwise fresh")
			}
			if err := startupcache.Write(entry, startupcache.SubdirDAP); err != nil {
				t.Fatal(err)
			}
			if _, _, _, err := loadDAPStartupState(root); err != nil {
				t.Fatal(err)
			}
			rebuilt, err := startupcache.Read(root, startupcache.SubdirDAP)
			if err != nil || rebuilt == nil || rebuilt.Version != dapCacheVersion || rebuilt.Version == legacyVersion || rebuilt.BuiltAt == "stale-flow-formula-model" {
				t.Fatalf("legacy DAP cache not replaced: entry=%v err=%v", rebuilt != nil, err)
			}
			rebuilt.BuiltAt = "warm-flow-formula-model"
			if err := startupcache.Write(rebuilt, startupcache.SubdirDAP); err != nil {
				t.Fatal(err)
			}
			org, runtime, _, err := loadDAPStartupState(root)
			if err != nil {
				t.Fatal(err)
			}
			warm, err := startupcache.Read(root, startupcache.SubdirDAP)
			if err != nil || warm == nil || warm.BuiltAt != "warm-flow-formula-model" {
				t.Fatal("new DAP cache was not reused")
			}
			want := map[string]map[string]any{}
			for _, line := range strings.Split(strings.TrimSpace(string(rows)), "\n")[1:] {
				cells := strings.SplitN(line, "\t", 4)
				if len(cells) == 4 && cells[0] == api && cells[1] == "anonymous" {
					var value map[string]any
					if err := json.Unmarshal([]byte(cells[3]), &value); err != nil {
						t.Fatal(err)
					}
					want[cells[2]] = value
				}
			}
			captureSource, err := os.ReadFile(filepath.Join(root, "capture.apex"))
			if err != nil {
				t.Fatal(err)
			}
			program, err := vm.CompileAnonymousWithOptions(string(captureSource), vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			machine := vm.New(nil)
			machine.SetOrg(&org)
			machine.SetCurrentNamespace(org.Namespace)
			machine.SetToolingExecuteAnonymous(true)
			if err := apextest.RegisterCompiledProjectRuntimeForRequest(machine, runtime); err != nil {
				t.Fatal(err)
			}
			_, err = machine.Execute(program)
			var runtimeErr *vm.RuntimeError
			if !errors.As(err, &runtimeErr) || !strings.HasSuffix(runtimeErr.Type, "AssertException") {
				t.Fatalf("missing captured observation: %v", err)
			}
			message := runtimeErr.ExceptionMessage()
			marker := strings.Index(message, "FFFRAME|")
			if marker < 0 {
				t.Fatalf("missing observation marker: %s", message)
			}
			var got map[string]map[string]any
			if err := json.Unmarshal([]byte(message[marker+len("FFFRAME|"):]), &got); err != nil {
				t.Fatalf("invalid observation %q: %v", message, err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("debugger warm replay=%#v, native=%#v", got, want)
			}
		})
	}
}
