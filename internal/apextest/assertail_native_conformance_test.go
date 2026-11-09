package apextest

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

// The anonymous rows run on an org the way glade exec builds it.
func applyConformanceScratchCountry(t *testing.T, org *storage.OrgState, root string) {
	t.Helper()
	loaded, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	ApplyScratchCountryAnonymousOrg(org, loaded)
}

// Families captured in one scratch org for the corpus assertion long tail.
// Each family has an org.tsv from native Salesforce (anonymous and isTest).

func TestJWSMissingCertificateNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "jws_missing_certificate", "JwsCertificateProbeTest", "62.0", "67.0")
}

func TestSOQLInNullCollectionNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "soql_in_null_collection", "NullBindProbeTest", "62.0", "67.0")
}

func TestLeadConvertFieldMappingNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "lead_convert_field_mapping", "LeadConvertMapProbeTest", "62.0", "67.0")
}

func TestScratchCountryLocaleNativeConformance(t *testing.T) {
	// The country in the scratch definition sets the default user's locale and
	// time zone on the named test route and on the anonymous routes alike.
	for _, country := range []string{"no", "de", "gb"} {
		t.Run(country, func(t *testing.T) {
			runInnerTypeNativeRoutes(t, "scratch_country_locale_"+country, "CountryLocaleProbeTest", []string{"anonymous", "isTest"}, nil, "62.0", "67.0")
		})
	}
}

func TestIDLiteralConversionNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "id_literal_conversion", "IdConversionProbeTest", "51.0", "52.0", "53.0", "54.0", "55.0", "62.0", "67.0")
}

func TestDateValueOfTimeTextNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "date_valueof_time_text", "DateValueOfProbeTest", "62.0", "67.0")
}

func TestSObjectEmptyStringFieldsNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "sobject_empty_string_fields", "EmptyStringProbeTest", "62.0", "67.0")
}

func TestTriggerEmptyStringValuesNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "trigger_empty_string_values", "EmptyTriggerProbeTest", "62.0", "67.0")
}

func TestGlobalDescribeSizeNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "global_describe_size", "GlobalDescribeSizeProbeTest", "62.0", "67.0")
}

func TestStubParamTypesNamespacedNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "stub_param_types_namespaced", "StubProbeTest", "62.0", "67.0")
}

func TestInactiveOwnerDMLNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "inactive_owner_dml", "InactiveOwnerProbeTest", "62.0", "67.0")
}

func TestAccessLevelRenderingNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "access_level_rendering", "AccessLevelRenderProbeTest", "62.0", "67.0")
}

func TestQueueableAccessLevelIdentityNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "queueable_accesslevel_identity", "QueueableAccessProbeTest", "62.0", "67.0")
}

func TestAuraRedirectNullNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "aura_redirect_null", "AuraRedirectProbeTest", "62.0", "67.0")
}

// Date.parse, Date.valueOf, Datetime.parse and Datetime.valueOf with locale
// text, ISO text and invalid text, for the scratch country (anonymous and
// named routes) and for runAs users of de_DE, en_GB and no_NO.
func TestDateParseLocalesNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "date_parse_locales", "DateParseLocaleProbeTest", "62.0", "67.0")
}

// The same locale rows hold when a project sets test.user.locale instead of a
// scratch country (the runAs rows of date_parse_locales, API 67.0).
func TestDateParseLocalesConfiguredUserLocale(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "date_parse_locales")
	data, err := os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	var named map[string][]string
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n")[1:] {
		cols := strings.SplitN(line, "\t", 3)
		if cols[0] == "67.0" && cols[1] == "isTest" {
			if err := json.Unmarshal([]byte(cols[2]), &named); err != nil {
				t.Fatal(err)
			}
		}
	}
	if len(named) != 3 {
		t.Fatalf("expected three captured locales, got %d", len(named))
	}
	for locale, want := range named {
		t.Run(locale, func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(dir, "project")
			err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				rel, err := filepath.Rel(source, path)
				if err != nil || rel == filepath.Join("config", "project-scratch-def.json") {
					return err
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, rel), strings.ReplaceAll(string(body), "62.0", "67.0"))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(root, "glade.yml"), "test:\n  user:\n    locale: "+locale+"\n")
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/CurrentLocaleProbeTest.cls"),
				"@IsTest public class CurrentLocaleProbeTest { @IsTest static void observe() { System.assert(false, 'OBS|' + JSON.serialize(DateParseLocaleProbe.observeCurrent())); } }")
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/CurrentLocaleProbeTest.cls-meta.xml"),
				`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>`)
			index := loadTestIndex(t, root)
			run := Run(index, Options{SelectedClasses: []string{"CurrentLocaleProbeTest"}, SelectedMethod: "observe", NoDiskCache: true, Parallelism: 1})
			if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
				t.Fatalf("expected one observation: %+v", run.Summary())
			}
			_, observed, _ := strings.Cut(run.Suites[0].Cases[0].Problem.Message, "OBS|")
			var got []string
			if err := json.Unmarshal([]byte(observed), &got); err != nil {
				t.Fatalf("invalid observation %q: %v", observed, err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("native=%v\nGlade=%v", want, got)
			}
		})
	}
}

// Native observations: testdata/conformance/blank_text_formula/org.tsv. Text
// formulas that evaluate to an empty string, to a single space and to a
// constant, read on trigger records, in memory and from SOQL.
func TestBlankTextFormulaNativeConformance(t *testing.T) {
	runInnerTypeNativeRoutesParts(t, "blank_text_formula", "FmProbeTest", true, []string{"anonymous", "isTest"}, compareFormulaObservationLeaves, "62.0", "67.0")
}

// walkObservationDifferences appends one line per differing leaf of two
// decoded JSON observations.
func walkObservationDifferences(path string, got, want any, differences *[]string) {
	gotMap, gotOK := got.(map[string]any)
	wantMap, wantOK := want.(map[string]any)
	if !gotOK || !wantOK {
		if !reflect.DeepEqual(got, want) {
			*differences = append(*differences, fmt.Sprintf("%s: native=%v glade=%v", path, want, got))
		}
		return
	}
	keys := map[string]bool{}
	for key := range gotMap {
		keys[key] = true
	}
	for key := range wantMap {
		keys[key] = true
	}
	sorted := make([]string, 0, len(keys))
	for key := range keys {
		sorted = append(sorted, key)
	}
	sort.Strings(sorted)
	for _, key := range sorted {
		walkObservationDifferences(path+"/"+key, gotMap[key], wantMap[key], differences)
	}
}

// compareFormulaObservationLeaves asserts every leaf of a formula observation
// (blank_text_formula, trigger_record_formulas): trigger records read their
// formulas on the trigger surface from their own snapshot
// (vm.applyTriggerFormulaSurface, dml.FormulaEvaluationOptions.TriggerRecord).
func compareFormulaObservationLeaves(t *testing.T, got, want any) {
	t.Helper()
	var differences []string
	walkObservationDifferences("", got, want, &differences)
	sort.Strings(differences)
	if len(differences) > 0 {
		t.Fatalf("%d differences:\n%s", len(differences), strings.Join(differences, "\n"))
	}
}
