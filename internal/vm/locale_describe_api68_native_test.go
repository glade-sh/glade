package vm

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

type localeNativeEntry struct {
	Value   string `json:"value"`
	Label   string `json:"label"`
	Active  bool   `json:"active"`
	Default bool   `json:"defaultValue"`
}

func localeNativeDescribeRows(t *testing.T) map[string][]localeNativeEntry {
	t.Helper()
	f, err := os.Open(filepath.Join("..", "dml", "testdata", "locale_sid_key", "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 4096), 1<<20)
	out := map[string][]localeNativeEntry{}
	for scanner.Scan() {
		row := strings.SplitN(scanner.Text(), "\t", 3)
		if len(row) != 3 || row[0] == "source_api" {
			continue
		}
		var payload struct {
			Entries []localeNativeEntry `json:"entries"`
		}
		if err := json.Unmarshal([]byte(row[2]), &payload); err != nil {
			t.Fatal(err)
		}
		out[row[0]] = payload.Entries
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// Native source and transport controls: ../dml/testdata/locale_sid_key/org.tsv:2-4
// and ../dml/testdata/locale_sid_key/cross-version/org.tsv.
func TestNativeLocaleDescribeVersioned(t *testing.T) {
	want := localeNativeDescribeRows(t)
	for _, api := range []string{"62.0", "63.0", "64.0", "65.0", "66.0", "67.0", "68.0"} {
		expectedAPI := api
		if api == "63.0" || api == "64.0" || api == "65.0" || api == "66.0" {
			expectedAPI = "67.0"
		}
		t.Run(api, func(t *testing.T) {
			org := storage.NewOrgState()
			machine := New(nil)
			machine.SetOrg(&org)
			machine.currentMethod = Method{APIVersion: api, SourceContextBound: true}
			desc, err := machine.describeFieldValue("User", "LocaleSidKey")
			if err != nil {
				t.Fatal(err)
			}
			var got []localeNativeEntry
			for _, e := range desc.Fields["picklistValues"].List {
				got = append(got, localeNativeEntry{Value: e.Fields["value"].Text, Label: e.Fields["label"].Text, Active: e.Fields["active"].Bool, Default: e.Fields["defaultValue"].Bool})
			}
			if !reflect.DeepEqual(got, want[expectedAPI]) {
				t.Fatalf("API %s locale describe differs: got %d entries, native %d", api, len(got), len(want[expectedAPI]))
			}
		})
	}
}

func TestNativeLocaleDescribeFirstVersionRetained(t *testing.T) {
	want := localeNativeDescribeRows(t)
	for _, first := range []string{"62.0", "68.0"} {
		t.Run(first, func(t *testing.T) {
			org := storage.NewOrgState()
			machine := New(nil)
			machine.SetOrg(&org)
			machine.currentMethod = Method{APIVersion: first, SourceContextBound: true}
			firstDesc, err := machine.describeFieldValue("User", "LocaleSidKey")
			if err != nil {
				t.Fatal(err)
			}
			other := "62.0"
			if first == other {
				other = "68.0"
			}
			machine.currentMethod = Method{APIVersion: other, SourceContextBound: true}
			secondDesc, err := machine.describeFieldValue("User", "LocaleSidKey")
			if err != nil {
				t.Fatal(err)
			}
			if len(firstDesc.Fields["picklistValues"].List) != len(want[first]) || len(secondDesc.Fields["picklistValues"].List) != len(want[first]) {
				t.Fatalf("first source %s: first=%d second=%d native=%d", first, len(firstDesc.Fields["picklistValues"].List), len(secondDesc.Fields["picklistValues"].List), len(want[first]))
			}
		})
	}
}

// Local scope control: explicitly supplied project picklists must remain intact.
func TestNativeLocaleDescribePreservesExplicitPicklist(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["User"] = storage.ObjectState{Definition: storage.ObjectDefinition{
		APIName: "User", Fields: map[string]storage.Field{
			"LocaleSidKey": {APIName: "LocaleSidKey", Type: storage.FieldPicklist, RestrictedPicklist: true, PicklistValuesConfigured: true,
				PicklistValues: []storage.PicklistValue{{Value: "custom", Label: "Custom", Active: true}}},
		},
	}}
	machine := New(nil)
	machine.SetOrg(&org)
	machine.currentMethod = Method{APIVersion: "68.0", SourceContextBound: true}
	desc, err := machine.describeFieldValue("User", "LocaleSidKey")
	if err != nil {
		t.Fatal(err)
	}
	values := desc.Fields["picklistValues"].List
	if len(values) != 1 || values[0].Fields["value"].Text != "custom" {
		t.Fatalf("explicit User locale list changed: %#v", values)
	}
}

// A describe must not mutate the catalog or the org used by DML.
func TestNativeLocaleDescribeDoesNotMutateSchema(t *testing.T) {
	before, ok := storage.StandardObjectDefinition("User")
	if !ok {
		t.Fatal("missing User catalog")
	}
	expected := append([]storage.PicklistValue(nil), before.Fields["LocaleSidKey"].PicklistValues...)
	org := storage.NewOrgState()
	storage.EnsureDeterministicPlatformData(&org)
	storage.EnsureStandardObject(&org, "User")
	machine := New(nil)
	machine.SetOrg(&org)
	machine.currentMethod = Method{APIVersion: "62.0", SourceContextBound: true}
	if _, err := machine.describeFieldValue("User", "LocaleSidKey"); err != nil {
		t.Fatal(err)
	}
	after, _ := storage.StandardObjectDefinition("User")
	if !reflect.DeepEqual(after.Fields["LocaleSidKey"].PicklistValues, expected) {
		t.Fatal("describe mutated standard catalog")
	}
	if user := org.Objects["User"]; !reflect.DeepEqual(user.Definition.Fields["LocaleSidKey"].PicklistValues, expected) {
		t.Fatal("describe mutated org schema")
	}
	engine := dml.NewEngine(&org)
	for _, locale := range []string{"en_BR", "en_MX", "en_SA"} {
		result := engine.Update([]storage.Record{{ID: storage.DefaultUserID, Object: "User", Fields: map[string]storage.Value{"LocaleSidKey": storage.StringValue(locale)}}})
		if len(result) != 1 || !result[0].Success {
			t.Fatalf("DML after API 62 describe for %s: %#v", locale, result)
		}
	}
}
