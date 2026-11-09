package apextest

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestR6FlowWarmProjectConformance(t *testing.T) {
	wasDisabled := disableDiskCache.Load()
	disableDiskCache.Store(false)
	InvalidateRuntimeCaches()
	t.Cleanup(func() { disableDiskCache.Store(wasDisabled); InvalidateRuntimeCaches() })
	for _, tc := range []struct {
		fixture, class string
		count          int
	}{
		{"r6flow_profile", "R6ProbeTest", 2},
		{"r6flow_user", "R6UserTest", 2},
		{"r6flow_permission", "R6PermissionTest", 2},
		{"r6flow_metadata", "R6MetadataTest", 2},
		{"r6flow_loop", "R6LoopTest", 2},
		{"r6flow_multi", "R6MultiTest", 3},
	} {
		for _, api := range []string{"62", "67"} {
			t.Run(tc.fixture+"/"+api, func(t *testing.T) {
				root := t.TempDir()
				copyDiskRuntimeScopeProject(t, filepath.Join("testdata", "conformance", tc.fixture, "api"+api), root)
				index := loadTestIndex(t, root)
				InvalidateRuntimeCaches()
				opts := Options{SelectedClasses: []string{tc.class}, Parallelism: 1}
				for _, phase := range []string{"cold", "warm"} {
					if phase == "warm" {
						InvalidateRuntimeCaches()
						if _, ok := tryLoadDiskRuntime(index); !ok {
							t.Fatal("runtime was not restored from disk")
						}
					}
					run := Run(index, opts)
					if s := run.Summary(); s.Total != tc.count || s.Passed != tc.count {
						t.Fatalf("%s: %s", phase, firstRunProblem(run))
					}
				}
			})
		}
	}
	fixture := filepath.Join("testdata", "conformance", "r6flow_expiry")
	f, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows[1:] {
		if row[1] != "isTest" || strings.HasPrefix(row[3], "SETUP|") {
			continue
		}
		row := row
		t.Run("expiry/"+row[0]+"/"+row[2], func(t *testing.T) {
			root := t.TempDir()
			copyDiskRuntimeScopeProject(t, filepath.Join(fixture, "api"+strings.TrimSuffix(row[0], ".0")), root)
			index := loadTestIndex(t, root)
			InvalidateRuntimeCaches()
			for _, phase := range []string{"cold", "warm"} {
				if phase == "warm" {
					InvalidateRuntimeCaches()
					if _, ok := tryLoadDiskRuntime(index); !ok {
						t.Fatal("runtime was not restored from disk")
					}
				}
				run := Run(index, Options{SelectedClasses: []string{"R6ExpiryTest"}, SelectedMethod: row[2], Parallelism: 1})
				if run.Summary().Total != 1 {
					t.Fatalf("%s: selection %+v", phase, run.Summary())
				}
				message := firstRunProblem(run)
				_, got, ok := strings.Cut(message, "R6|")
				if !ok || got != row[3] {
					t.Fatalf("%s: native=%q local=%q", phase, row[3], message)
				}
			}
		})
	}
}
