package apextest

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

func TestStandardDescribeProfileNativeFields(t *testing.T) {
	const dir = "testdata/conformance/standard_describe_profiles/"
	var plan struct {
		Samples               map[string][]string
		HistoricalExtraFields []struct{ Object, Field string } `json:"historical_extra_fields"`
	}
	raw, err := os.ReadFile(dir + "cases.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &plan); err != nil {
		t.Fatal(err)
	}
	if len(plan.HistoricalExtraFields) != 416 {
		t.Fatal("incomplete field recapture case table")
	}
	for _, api := range []string{"62", "67"} {
		t.Run(api+"/storage-describe", func(t *testing.T) {
			file, err := os.Open(dir + "P0/api" + api + "/granted/describes.json.gz")
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			stream, err := gzip.NewReader(file)
			if err != nil {
				t.Fatal(err)
			}
			defer stream.Close()
			var bundle struct {
				Describes map[string]struct{ Fields []struct{ Name string } }
			}
			if err := json.NewDecoder(stream).Decode(&bundle); err != nil {
				t.Fatal(err)
			}
			wanted := map[string]map[string]bool{}
			for object, sampleFields := range plan.Samples {
				wanted[object] = map[string]bool{}
				for _, field := range sampleFields {
					if field != "*" {
						wanted[object][field] = true
					}
				}
			}
			for _, row := range plan.HistoricalExtraFields {
				if wanted[row.Object] == nil {
					wanted[row.Object] = map[string]bool{}
				}
				wanted[row.Object][row.Field] = true
			}
			for object, fields := range wanted {
				t.Run(object, func(t *testing.T) {
					captured, observed := bundle.Describes[object]
					if !observed {
						t.Log("no successful object describe in this profile; field gates remain unproven")
						return
					}
					native := standardDescribeNativeFieldMembership(t, dir, api, object, captured.Fields)
					definition, _ := storage.StandardObjectDefinition(object)
					storage.EnsureStandardObjectFieldsForFeatures(&definition, nil)
					var mismatches []string
					for field := range fields {
						_, local := storage.ResolveFieldName(definition, "", field)
						if local != native[field] {
							mismatches = append(mismatches, fmt.Sprintf("%s: native=%t Glade=%t", field, native[field], local))
						}
					}
					if len(mismatches) != 0 {
						sort.Strings(mismatches)
						t.Fatalf("%d captured field mismatches: %s", len(mismatches), strings.Join(mismatches, "; "))
					}
				})
			}
		})
	}
}

type standardDescribeObservation struct {
	SourceSHA256                                        string `json:"source_sha256"`
	ID, API, Route, Context, Source, ClassName, Message string
	Compiled                                            bool
}

func standardDescribeObservations(t *testing.T) []standardDescribeObservation {
	t.Helper()
	return standardDescribeProfileObservations(t, "P0")
}

func standardDescribeProfileObservations(t *testing.T, profile string) []standardDescribeObservation {
	t.Helper()
	dir := filepath.Join("testdata/conformance/standard_describe_profiles", profile)
	var observations []standardDescribeObservation
	for _, api := range []string{"62", "67"} {
		for _, context := range []string{"default", "granted"} {
			raw, err := os.ReadFile(filepath.Join(dir, "api"+api, context, "slices.json"))
			if err != nil {
				t.Fatal(err)
			}
			var rows []standardDescribeObservation
			if err := json.Unmarshal(raw, &rows); err != nil {
				t.Fatal(err)
			}
			observations = append(observations, rows...)
		}
	}
	name := "org.tsv"
	if profile != "P0" {
		name = "slice-org.tsv"
	}
	file, err := os.Open(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != len(observations)+1 || strings.Join(rows[0], ",") != "api,context,route,id,compiled,message,source_sha256" {
		t.Fatal("incomplete native describe observations")
	}
	seen := map[string]bool{}
	for i, row := range observations {
		want := []string{row.API, row.Context, row.Route, row.ID, strconv.FormatBool(row.Compiled), row.Message, row.SourceSHA256}
		if strings.Join(rows[i+1], "\t") != strings.Join(want, "\t") {
			t.Fatalf("native row %d differs from captured source", i+1)
		}
		key := strings.Join(want[:4], "/")
		if seen[key] || row.Source == "" {
			t.Fatalf("duplicate or empty native source: %s", key)
		}
		digest := sha256.Sum256([]byte(row.Source))
		if hex.EncodeToString(digest[:]) != row.SourceSHA256 {
			t.Fatalf("captured source changed: %s", key)
		}
		seen[key] = true
	}
	return observations
}

func TestStandardDescribeProfileNativeCaptureIntegrity(t *testing.T) {
	for _, profile := range []string{"P0", "P11"} {
		t.Run(profile, func(t *testing.T) {
			rows := standardDescribeProfileObservations(t, profile)
			for _, api := range []string{"62.0", "67.0"} {
				for _, context := range []string{"default", "granted"} {
					anonymous, named, runnable, executed := 0, 0, 0, 0
					for _, row := range rows {
						if row.API != api || row.Context != context {
							continue
						}
						switch row.Route {
						case "anonymous":
							anonymous++
						case "named":
							named++
							if row.Compiled {
								runnable++
							}
						case "test-runner":
							executed++
						default:
							t.Fatalf("unknown captured route: %s", row.Route)
						}
					}
					want := 15
					if context == "granted" {
						want = 21
					}
					if anonymous != want || named != want || executed != runnable || runnable == 0 {
						t.Fatalf("incomplete %s/%s route matrix: anonymous=%d named=%d runnable=%d executed=%d", api, context, anonymous, named, runnable, executed)
					}
				}
			}

		})
	}
}

func TestStandardDescribeProfileNativeCompile(t *testing.T) {
	for _, row := range standardDescribeObservations(t) {
		// Default-profile rows measure Salesforce FLS. Glade's standard schema
		// represents the explicit-field-access view used by named Apex.
		if row.Context != "granted" || row.Route == "test-runner" {
			continue
		}
		t.Run(row.API+"/"+row.Route+"/"+row.ID, func(t *testing.T) {
			classes := map[string]string{}
			if row.Route == "named" {
				classes[row.ClassName] = row.Source
			}
			index := standardDescribeDeclaredBoundaryIndex(t, row.API, classes, nil)
			analysis := sema.Analyze(index)
			if row.Route == "anonymous" {
				analysis = sema.AnalyzeAnonymous(index, row.Source, row.API)
			}
			compiled := !index.HasErrors() && !analysis.HasErrors()
			if compiled != row.Compiled {
				t.Fatalf("native compiled=%t (%s); Glade compiled=%t: %v %v", row.Compiled, row.Message, compiled, index.Diagnostics, analysis.Diagnostics)
			}
		})
	}
}

func TestStandardDescribeProfileNativeAnonymousRuntime(t *testing.T) {
	for _, row := range standardDescribeObservations(t) {
		if row.Context != "granted" || row.Route != "anonymous" || !row.Compiled {
			continue
		}
		t.Run(row.API+"/anonymous/"+row.ID, func(t *testing.T) {
			index := standardDescribeDeclaredBoundaryIndex(t, row.API, nil, nil)
			program, err := vm.CompileAnonymousWithOptions(row.Source, vm.CompileOptions{APIVersion: row.API})
			if err != nil {
				t.Fatal(err)
			}
			org := orgFromIndex(index)
			org.APIVersion = row.API
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			result, err := runner.execute(program)
			if err != nil {
				t.Fatal(err)
			}
			want := "RESULT|" + row.Message
			if len(result.Debug) != 1 || result.Debug[0] != want {
				t.Fatalf("native %q; Glade %v", want, result.Debug)
			}
		})
	}
}

func TestStandardDescribeProfileNativeProjectTestRunner(t *testing.T) {
	for _, row := range standardDescribeObservations(t) {
		if row.Context != "granted" || row.Route != "test-runner" {
			continue
		}
		t.Run(row.API+"/test-runner/"+row.ID, func(t *testing.T) {
			index := standardDescribeDeclaredBoundaryIndex(t, row.API, map[string]string{row.ClassName: row.Source}, nil)
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native project compiled; Glade rejected: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			run := Run(index, Options{SelectedClasses: []string{row.ClassName}, SelectedMethod: "sample", NoDiskCache: true, Parallelism: 1})
			if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
				t.Fatalf("expected one captured project case: %+v", run.Summary())
			}
			problem := run.Suites[0].Cases[0].Problem
			if problem == nil || !strings.HasSuffix(problem.Type, "AssertException") || !strings.HasSuffix(problem.Message, "OBSERVED|"+row.Message) {
				t.Fatalf("native OBSERVED|%s; Glade %+v", row.Message, problem)
			}
		})
	}
}

// Named Apex can expose non-permissionable fields omitted by granted REST.
// Use positive observations from both routes; REST absence is not a rejection.
func standardDescribeNativeFieldMembership(t *testing.T, dir, api, object string, fields []struct{ Name string }) map[string]bool {
	t.Helper()
	native := map[string]bool{}
	for _, field := range fields {
		native[field.Name] = true
	}
	file, err := os.Open(filepath.Join(dir, "P0", "api"+api, "default", "named-fields.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows[1:] {
		if row[0] == object && row[1] != "" && row[2] == "true" {
			native[row[1]] = true
		}
	}

	return native
}

func TestStandardDescribeProfileNativeNamedMapMembershipRoutes(t *testing.T) {
	const dir = "testdata/conformance/standard_describe_profiles/"
	for _, api := range []string{"62", "67"} {
		t.Run(api+"/named-map-versus-rest", func(t *testing.T) {
			file, err := os.Open(dir + "P0/api" + api + "/default/named-fields.tsv")
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			reader := csv.NewReader(file)
			reader.Comma = '\t'
			rows, err := reader.ReadAll()
			if err != nil {
				t.Fatal(err)
			}
			observed := false
			for _, row := range rows[1:] {
				if row[0] == "Opportunity" && row[1] == "IqScore" && row[2] == "true" {
					observed = true
				}
			}
			if !observed {
				t.Fatal("missing native named-map presence row")
			}
			// An empty REST field list cannot disprove a captured named-map field.
			fields := standardDescribeNativeFieldMembership(t, dir, api, "Opportunity", nil)
			if !fields["IqScore"] {
				t.Fatal("REST absence incorrectly overrides native named-map presence for Opportunity.IqScore")
			}
			if fields["SyntheticAbsentField"] {
				t.Fatal("uncaptured field became present")
			}
			summary, err := os.Open(dir + "P0/historical-extra-fields.tsv")
			if err != nil {
				t.Fatal(err)
			}
			defer summary.Close()
			reader = csv.NewReader(summary)
			reader.Comma = '\t'
			rows, err = reader.ReadAll()
			if err != nil {
				t.Fatal(err)
			}
			if strings.Join(rows[0], ",") != "api,object,field,rest_observation,named_map_present,effective_membership" {
				t.Fatal("historical summary must preserve both REST absence and named-map presence")
			}
			found := false
			for _, row := range rows[1:] {
				if row[0] == api+".0" && row[1] == "Opportunity" && row[2] == "IqScore" {
					found = true
					if row[3] != "absent-from-granted-describe" || row[4] != "true" || row[5] != "present" {
						t.Fatalf("incorrect named-only historical summary: %v", row)
					}
				}
			}
			if !found {
				t.Fatal("missing historical IqScore observation")
			}

		})
	}
}

func TestStandardDescribeProfileNativeExportPrivacyRoutes(t *testing.T) {
	var paths []string
	err := filepath.WalkDir("testdata/conformance/standard_describe_profiles", func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && entry.Name() == "describes.json.gz" {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatal("no describe exports")
	}
	for _, path := range paths {
		t.Run(strings.TrimSuffix(path, "/describes.json.gz"), func(t *testing.T) {
			file, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			stream, err := gzip.NewReader(file)
			if err != nil {
				t.Fatal(err)
			}
			defer stream.Close()
			var bundle struct{ Describes map[string]json.RawMessage }
			if err := json.NewDecoder(stream).Decode(&bundle); err != nil {
				t.Fatal(err)
			}
			for object, body := range bundle.Describes {
				if strings.Contains(string(body), ".scratch.my.salesforce.com") || strings.Contains(string(body), ".scratch.my.site.com") {
					t.Errorf("%s retains a scratch hostname in describe metadata", object)
				}
			}
		})
	}
}

func TestStandardDescribeProfileNativeFeatureCaptureMatrix(t *testing.T) {
	const dir = "testdata/conformance/standard_describe_profiles/"
	file, err := os.Open(dir + "profile-status.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 14 {
		t.Fatalf("expected thirteen profiles, got %d", len(rows)-1)
	}
	for _, row := range rows[1:] {
		t.Run(row[0], func(t *testing.T) {
			if row[1] != "captured" {
				t.Fatalf("profile remains %s", row[1])
			}
			definition, err := os.ReadFile(filepath.Join(dir, "profiles", row[0]+".json"))
			if err != nil {
				t.Fatal(err)
			}
			digest := sha256.Sum256(definition)
			if len(row) != 11 || row[5] != hex.EncodeToString(digest[:]) {
				t.Fatal("profile definition is not bound to the captured status")
			}
			if _, err := os.Stat(dir + row[0] + "/org.tsv"); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// Feature deltas are observations between isolated orgs, not runtime gate rules.
func TestStandardDescribeProfileNativeFeatureSetDifferences(t *testing.T) {
	const dir = "testdata/conformance/standard_describe_profiles/"
	file, err := os.Open(dir + "feature-set-differences.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) < 2 || strings.Join(rows[0], ",") != "api,profile,baseline_profile,route,object,field,change" {
		t.Fatal("missing feature set differences")
	}
	actual := map[string]bool{}
	for _, row := range rows[1:] {
		key := strings.Join(row, "\t")
		if actual[key] {
			t.Fatalf("duplicate delta %s", key)
		}
		actual[key] = true
	}
	expected := map[string]bool{}
	for _, api := range []string{"62", "67"} {
		for _, route := range []string{"anonymous-default", "anonymous-granted", "named-default", "effective-membership"} {
			base := standardDescribeProfileFieldSet(t, dir, "P0", api, route)
			for _, profile := range []string{"P1", "P2", "P3", "P4", "P5", "P6", "P7", "P8", "P9", "P10", "P11", "PU"} {
				t.Run(api+"/"+profile+"/"+route, func(t *testing.T) {
					baselineProfile := "P0"
					baseline := base
					if profile == "P10" {
						baselineProfile = "P4"
						baseline = standardDescribeProfileFieldSet(t, dir, baselineProfile, api, route)
					}
					fields := standardDescribeProfileFieldSet(t, dir, profile, api, route)
					for field := range fields {
						if !baseline[field] {
							expected[api+".0\t"+profile+"\t"+baselineProfile+"\t"+route+"\t"+field+"\tadded"] = true
						}
					}
					for field := range baseline {
						if !fields[field] {
							expected[api+".0\t"+profile+"\t"+baselineProfile+"\t"+route+"\t"+field+"\tremoved"] = true
						}
					}
				})
			}
		}
	}
	for row := range expected {
		if !actual[row] {
			t.Errorf("missing observed difference: %s", row)
		}
	}
	for row := range actual {
		if !expected[row] {
			t.Errorf("unbacked difference: %s", row)
		}
	}
}

func standardDescribeProfileFieldSet(t *testing.T, dir, profile, api, route string) map[string]bool {
	t.Helper()
	context, name := "default", "apex-fields.tsv"
	if route == "anonymous-granted" {
		context = "granted"
	}
	if route == "named-default" || route == "effective-membership" {
		name = "named-fields.tsv"
	}
	file, err := os.Open(filepath.Join(dir, profile, "api"+api, context, name))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	fields := map[string]bool{}
	objects := map[string]bool{}
	for _, row := range rows[1:] {
		objects[row[0]] = true
		if row[2] == "true" && row[1] != "" {
			fields[row[0]+"\t"+row[1]] = true
		}
	}
	if route == "effective-membership" {
		bundleFile, err := os.Open(filepath.Join(dir, profile, "api"+api, "granted", "describes.json.gz"))
		if err != nil {
			t.Fatal(err)
		}
		defer bundleFile.Close()
		stream, err := gzip.NewReader(bundleFile)
		if err != nil {
			t.Fatal(err)
		}
		defer stream.Close()
		var bundle struct {
			Describes map[string]struct{ Fields []struct{ Name string } }
		}
		if err := json.NewDecoder(stream).Decode(&bundle); err != nil {
			t.Fatal(err)
		}
		for object, body := range bundle.Describes {
			if objects[object] {
				for _, field := range body.Fields {
					fields[object+"\t"+field.Name] = true
				}
			}
		}
	}
	return fields
}

func TestStandardDescribeProfileNativeCommercePrerequisite(t *testing.T) {
	const dir = "testdata/conformance/standard_describe_profiles/"
	raw, err := os.ReadFile(dir + "P10/capture-blocked.json")
	if err != nil {
		t.Fatal(err)
	}
	var rejection struct{ Name, Message string }
	if err := json.Unmarshal(raw, &rejection); err != nil {
		t.Fatal(err)
	}
	if rejection.Name != "MISSING_ARGUMENT" || !strings.Contains(rejection.Message, "COMMUNITIES") {
		t.Fatal("missing native Commerce prerequisite rejection")
	}
	raw, err = os.ReadFile(dir + "profiles/P10.json")
	if err != nil {
		t.Fatal(err)
	}
	var definition struct {
		Features []string
		Settings struct {
			CommunitiesSettings struct{ EnableNetworksEnabled bool }
		}
	}
	if err := json.Unmarshal(raw, &definition); err != nil {
		t.Fatal(err)
	}
	features := map[string]bool{}
	for _, feature := range definition.Features {
		features[feature] = true
	}
	if len(features) != 2 || !features["B2BCommerce"] || !features["Communities"] || !definition.Settings.CommunitiesSettings.EnableNetworksEnabled {
		t.Fatal("native requires Communities; Commerce must use the matched Communities baseline")
	}
}

// These captures explicitly describe a fresh Developer scratch org.
func standardDescribeDeclaredBoundaryIndex(t *testing.T, api string, classes, objects map[string]string) typesys.Index {
	t.Helper()
	index := nativeBoundaryIndex(t, api, classes, objects)
	writeFile(t, filepath.Join(index.Project.Root, "config/project-scratch-def.json"), `{"edition":"Developer"}`)
	return loadTestIndex(t, index.Project.Root)
}
