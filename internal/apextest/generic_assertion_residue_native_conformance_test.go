package apextest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

const genericAssertionResidueDirectory = "testdata/conformance/generic_assertion_residue"

func readGenericAssertionResidueRows(t *testing.T) ([][]string, error) {
	t.Helper()
	rows, err := os.ReadFile(filepath.Join(genericAssertionResidueDirectory, "org.tsv"))
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(rows)), "\n")
	const header = "source_api\tmap_json_order\tmap_key_set_order\tgrouped_aggregate_alias\tparent_projection_requery\tdate_format_no\tdate_format_no_NO\tdatetime_format_no\tdatetime_format_no_NO\tlocale_key_status"
	if len(lines) != 3 || lines[0] != header {
		return nil, os.ErrInvalid
	}
	parsed := make([][]string, 0, len(lines)-1)
	for _, line := range lines[1:] {
		columns := strings.Split(line, "\t")
		if len(columns) != 10 || (columns[0] != "62.0" && columns[0] != "67.0") || columns[9] != "no=accepted,no_NO=accepted,nb_NO=rejected" {
			return nil, os.ErrInvalid
		}
		parsed = append(parsed, columns)
	}
	return parsed, nil
}

func readGenericAssertionResidueObservations(t *testing.T, debug []string) map[string]string {
	t.Helper()
	observed := map[string]string{}
	for _, entry := range debug {
		message := strings.ReplaceAll(entry, "&#124;", "|")
		if _, payload, ok := strings.Cut(message, "OBS|"); ok {
			name, value, found := strings.Cut(payload, "|")
			if !found || observed[name] != "" {
				t.Fatalf("invalid or duplicate observation: %q", entry)
			}
			observed[name] = value
		}
	}
	return observed
}

func TestGenericAssertionResidueNativeConformance(t *testing.T) {
	rows, err := readGenericAssertionResidueRows(t)
	if err != nil {
		t.Fatal(err)
	}
	anonymous, err := os.ReadFile(filepath.Join(genericAssertionResidueDirectory, "anonymous.apex"))
	if err != nil {
		t.Fatal(err)
	}
	for _, columns := range rows {
		api := columns[0]
		t.Run(api, func(t *testing.T) {
			program, err := vm.CompileAnonymousWithOptions(string(anonymous), vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatalf("compile API %s: %v", api, err)
			}
			org := orgFromIndex(typesys.Index{})
			org.APIVersion = api
			machine := vm.New(nil)
			machine.SetOrg(&org)
			result, err := machine.Execute(program)
			if err != nil {
				t.Fatalf("execute API %s: %v", api, err)
			}
			observed := readGenericAssertionResidueObservations(t, result.Debug)
			if len(observed) != 4 {
				t.Fatalf("observations = %v, want four native markers", observed)
			}
			if observed["mapJsonOrder"] != columns[1] {
				t.Errorf("map JSON order native=%s; Glade=%s", columns[1], observed["mapJsonOrder"])
			}
			if observed["mapKeySetOrder"] != columns[2] {
				t.Errorf("map keySet order native=%s; Glade=%s", columns[2], observed["mapKeySetOrder"])
			}
			for _, observation := range []struct {
				name   string
				column int
			}{{"groupedAggregateAlias", 3}, {"parentProjectionRequery", 4}} {
				var native, local any
				if err := json.Unmarshal([]byte(columns[observation.column]), &native); err != nil {
					t.Fatalf("decode native %s: %v", observation.name, err)
				}
				if err := json.Unmarshal([]byte(observed[observation.name]), &local); err != nil {
					t.Fatalf("decode Glade %s=%q: %v", observation.name, observed[observation.name], err)
				}
				if !reflect.DeepEqual(native, local) {
					t.Errorf("%s native=%s; Glade=%s", observation.name, columns[observation.column], observed[observation.name])
				}
			}
		})
	}
}

func TestDateTimeFormatUsesNorwegianLocaleNativeConformance(t *testing.T) {
	rows, err := readGenericAssertionResidueRows(t)
	if err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile(filepath.Join(genericAssertionResidueDirectory, "locale.anonymous.apex"))
	if err != nil {
		t.Fatal(err)
	}
	for _, columns := range rows {
		api := columns[0]
		for _, locale := range []struct {
			name           string
			dateColumn     int
			datetimeColumn int
		}{{"no", 5, 7}, {"no_NO", 6, 8}} {
			t.Run(api+"/"+locale.name, func(t *testing.T) {
				program, err := vm.CompileAnonymousWithOptions(string(source), vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatalf("compile API %s: %v", api, err)
				}
				machine := vm.New(nil)
				machine.SetCurrentUser(storage.Record{
					ID:     "005-synthetic-locale",
					Object: "User",
					Fields: map[string]storage.Value{
						"LocaleSidKey":      storage.StringValue(locale.name),
						"LanguageLocaleKey": storage.StringValue("en_US"),
						"TimeZoneSidKey":    storage.StringValue("GMT"),
					},
				})
				result, err := machine.Execute(program)
				if err != nil {
					t.Fatalf("execute API %s locale %s: %v", api, locale.name, err)
				}
				var trace []string
				for _, entry := range result.Debug {
					message := strings.ReplaceAll(entry, "&#124;", "|")
					if strings.Contains(message, "TRACE|") || strings.Contains(message, "OBS|") {
						trace = append(trace, message)
					}
				}
				t.Logf("API %s locale %s debug markers: %s", api, locale.name, strings.Join(trace, " -> "))
				observed := readGenericAssertionResidueObservations(t, result.Debug)
				if len(observed) != 1 {
					t.Fatalf("observations = %v, want one native marker", observed)
				}
				var actual struct {
					Locale         string `json:"locale"`
					DateFormat     string `json:"dateFormat"`
					DatetimeFormat string `json:"datetimeFormat"`
				}
				if err := json.Unmarshal([]byte(observed["localeDateFormat"]), &actual); err != nil {
					t.Fatalf("decode Glade observation %q: %v", observed["localeDateFormat"], err)
				}
				if actual.Locale != locale.name {
					t.Fatalf("locale = %q, want %q", actual.Locale, locale.name)
				}
				if actual.DateFormat != columns[locale.dateColumn] {
					t.Errorf("Date.format() native=%q; Glade=%q", columns[locale.dateColumn], actual.DateFormat)
				}
				if actual.DatetimeFormat != columns[locale.datetimeColumn] {
					t.Errorf("Datetime.format() native=%q; Glade=%q", columns[locale.datetimeColumn], actual.DatetimeFormat)
				}
			})
		}
	}
}

// Execute the deployed native class through the project runner, including its
// persisted users and runAs scopes, rather than only injecting a VM user.
func TestNorwegianLocaleNamedProjectIsTestNativeRoutes(t *testing.T) {
	rows, err := readGenericAssertionResidueRows(t)
	if err != nil {
		t.Fatal(err)
	}
	index := loadTestIndex(t, filepath.Join(genericAssertionResidueDirectory, "project"))
	for _, columns := range rows {
		t.Run(columns[0]+"/named_project_istest", func(t *testing.T) {
			class := "LocaleFormatProbe" + strings.TrimSuffix(columns[0], ".0")
			run := Run(index, Options{SelectedClasses: []string{class}, SelectedMethod: "observe", NoDiskCache: true, Parallelism: 1})
			if summary := run.Summary(); summary.Total != 1 || summary.Failed != 1 || len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
				t.Fatalf("missing native locale observation: %+v %+v", summary, run.Suites)
			}
			message := strings.ReplaceAll(run.Suites[0].Cases[0].Problem.Message, "&#124;", "|")
			_, payload, ok := strings.Cut(message, "OBS|dateFormatLocale|")
			if !ok {
				t.Fatalf("missing locale capture: %q", message)
			}
			var got map[string]json.RawMessage
			if err := json.Unmarshal([]byte(payload), &got); err != nil {
				t.Fatal(err)
			}
			for _, locale := range []struct {
				name                       string
				dateColumn, datetimeColumn int
			}{{"no", 5, 7}, {"no_NO", 6, 8}} {
				var actual struct {
					Locale         string `json:"locale"`
					DateFormat     string `json:"dateFormat"`
					DatetimeFormat string `json:"datetimeFormat"`
				}
				if err := json.Unmarshal(got[locale.name], &actual); err != nil {
					t.Fatalf("locale %s=%s: %v", locale.name, got[locale.name], err)
				}
				if actual.Locale != locale.name || actual.DateFormat != columns[locale.dateColumn] || actual.DatetimeFormat != columns[locale.datetimeColumn] {
					t.Errorf("native locale=%s Date=%s Datetime=%s; project=%+v", locale.name, columns[locale.dateColumn], columns[locale.datetimeColumn], actual)
				}
			}
		})
	}
}
