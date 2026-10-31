package apextest

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
)

func TestSObjectJSON3NativeConformance(t *testing.T) {
	t.Run("projection", func(t *testing.T) {
		runSObjectJSON2NativeConformance(t, "testdata/conformance/sobject_json3", "SObjectJSON3Probe", "SOBJECT_JSON3|", "", "numberHigh numberZero numberNegative standardDouble standardZero standardNegative formatNumber formatAlias formatDate formatDateEndYear formatDateMidYear formatZeroCurrency formatNegativeCurrency formatNull toLabel toLabelAlias toLabelNull taskOwner unknownField invalidOwnerType emptyBoundary", "serialize", "suppress")
	})
	t.Run("owner", runSObjectJSON3OwnerNativeConformance)
}

func runSObjectJSON3OwnerNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/sobject_json3"
	file, err := os.Open(filepath.Join(fixture, "owner-org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	rows := map[string]map[string]string{}
	scanner := bufio.NewScanner(file)
	line := 0
	for scanner.Scan() {
		line++
		if line == 1 {
			continue
		}
		parts := strings.SplitN(scanner.Text(), "\t", 5)
		if len(parts) != 5 || parts[1] != "isTest" || parts[3] != "serialize" {
			t.Fatalf("invalid owner-org.tsv:%d", line)
		}
		var expected string
		if err := json.Unmarshal([]byte(parts[4]), &expected); err != nil {
			t.Fatal(err)
		}
		if rows[parts[0]] == nil {
			rows[parts[0]] = map[string]string{}
		}
		rows[parts[0]][parts[2]] = expected
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	releaseRuntimeCachesAfterTest(t)
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			if len(rows[api]) != 4 {
				t.Fatalf("expected four owner contexts, got %d", len(rows[api]))
			}
			root := t.TempDir()
			if err := os.CopyFS(root, os.DirFS(filepath.Join(fixture, "project"))); err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":".","default":true}]}`, api))
			writeFile(t, filepath.Join(root, "classes/SObjectJSON3Probe.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			capture, err := os.ReadFile(filepath.Join(fixture, "capture/SObjectJSON3OwnerCaptureTest.cls"))
			if err != nil {
				t.Fatal(err)
			}
			source := strings.ReplaceAll(string(capture), "SObjectJSON3OwnerCaptureTest", "SObjectJSON3OwnerTest")
			for shape, value := range rows[api] {
				before := "System.assert(false,'SOBJECT_OWNER|" + shape + "|'+JSON.serialize(SObjectJSON3Probe.observeOwner()));"
				after := "System.assertEquals(" + conformanceApexString(value) + ",SObjectJSON3Probe.observeOwner());"
				if !strings.Contains(source, before) {
					t.Fatalf("missing capture context %s", shape)
				}
				source = strings.Replace(source, before, after, 1)
			}
			writeFile(t, filepath.Join(root, "classes/SObjectJSON3OwnerTest.cls"), source)
			writeFile(t, filepath.Join(root, "classes/SObjectJSON3OwnerTest.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			InvalidateRuntimeCaches()
			for _, mode := range []string{"cold", "warm"} {
				t.Run(mode, func(t *testing.T) {
					run := Run(index, Options{SelectedClasses: []string{"SObjectJSON3OwnerTest"}, RuntimeRESTAPIVersion: "68.0", NoDiskCache: true, Parallelism: 1})
					if summary := run.Summary(); summary.Total != 4 || summary.Passed != 4 {
						t.Fatalf("owner native rows: %+v; %s", summary, firstRunProblem(run))
					}
				})
			}
		})
	}
}

// The route fixture uses the same locale and translation setup as the main
// capture, but keeps its independently captured observations separate.
func TestSObjectJSON3RouteNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/sobject_json3"
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS(fixture)); err != nil {
		t.Fatal(err)
	}
	rows, err := os.ReadFile(filepath.Join(root, "routes-org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "org.tsv"), rows, 0600); err != nil {
		t.Fatal(err)
	}
	runSObjectJSON2NativeConformance(t, root, "SObjectJSON3Routes", "SOBJECT_ROUTES|", "", "static locator child childLabel missing", "value")
}

// Boundary captures use a common French locale with independently activated
// French and German user languages. Each oracle is captured on both routes.
func TestSObjectJSON3BoundaryNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/sobject_json3"
	for _, language := range []string{"fr", "de"} {
		t.Run(language, func(t *testing.T) {
			root := t.TempDir()
			if err := os.CopyFS(root, os.DirFS(fixture)); err != nil {
				t.Fatal(err)
			}
			rows, err := os.ReadFile(filepath.Join(root, "boundary-"+language+"-org.tsv"))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "org.tsv"), rows, 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.CopyFS(filepath.Join(root, "project", "standardValueSetTranslations"), os.DirFS(filepath.Join(root, "boundary-metadata", "standardValueSetTranslations"))); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "project", "glade.yml"), []byte("org:\n  features: [TranslationLanguage-fr, TranslationLanguage-de]\n"), 0600); err != nil {
				t.Fatal(err)
			}
			runSObjectJSON2NativeConformanceWithLanguage(t, root, "SObjectJSON3BoundaryProbe", "SOBJECT_BOUNDARY|", "", "formatMoney formatNegative formatNull formatDate formatDateNegative labelAgriculture labelBanking labelTechnology labelNull stageProspecting stageQualification stageNeeds", language, "value")
		})
	}
}
