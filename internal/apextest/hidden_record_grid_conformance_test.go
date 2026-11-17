package apextest

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The SF-4 hidden-record grid: a default-mode update of a record the
// Standard User cannot see. Records created in the test or in @TestSetup report
// INSUFFICIENT_ACCESS_ON_CROSS_REFERENCE_ENTITY; records that already exist in
// the org under SeeAllData=true report INSUFFICIENT_ACCESS_OR_READONLY. The
// local test org holds no pre-existing records, so the SeeAllData rows
// (class names ending in C) cannot run.
func TestHiddenRecordGridNativeConformance(t *testing.T) {
	releaseRuntimeCachesAfterTest(t)
	dir := "testdata/conformance/hidden_record_grid"
	body, err := os.ReadFile(filepath.Join(dir, "rows.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	byAPI := map[string][][3]string{}
	for i, line := range strings.Split(strings.TrimSpace(string(body)), "\n") {
		if i == 0 {
			continue
		}
		c := strings.Split(line, "\t")
		byAPI[c[0]] = append(byAPI[c[0]], [3]string{c[1], c[2], c[3]})
	}
	apis := make([]string, 0, len(byAPI))
	for api := range byAPI {
		apis = append(apis, api)
	}
	sort.Strings(apis)
	for _, api := range apis {
		api := api
		t.Run(api, func(t *testing.T) {
			root := gapFamily{dir: dir, adminFrom: []string{"AdminAccess"}}.writeProject(t, api)
			index := loadTestIndex(t, root)
			mask := regexp.MustCompile(`\b[a-zA-Z0-9]{15,18}\b`)
			for _, row := range byAPI[api] {
				class, method, want := row[0], row[1], row[2]
				t.Run(class+"."+method, func(t *testing.T) {
					if strings.HasSuffix(class, "C") {
						t.Logf("SeeAllData rows read records that already exist in the org; the local test org holds none; not replayed")
						return
					}
					run := Run(index, Options{SelectedClasses: []string{class}, SelectedMethod: method, NoDiskCache: true, Parallelism: 1})
					var got string
					for _, suite := range run.Suites {
						for _, entry := range suite.Cases {
							if entry.Problem == nil {
								got = "pass"
							} else {
								got = entry.Problem.Type + ": " + entry.Problem.Message
							}
						}
					}
					got = strings.TrimPrefix(got, "System.AssertException: Assertion Failed: ")
					if mask.MatchString(want) && got == "" {
						t.Fatal("no result")
					}
					if got != want {
						t.Fatalf("native=%q\nglade =%q", want, got)
					}
				})
			}
		})
	}
}
