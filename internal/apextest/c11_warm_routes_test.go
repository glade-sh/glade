package apextest_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
)

type c11WarmCase struct {
	name, source, class, method, arm string
	files                            map[string]string
}

// Cold and warm runs must produce the same status and message, whether the
// actor prerequisite is carried or the captured operation is reached.
func TestC11WarmActorGateRoutes(t *testing.T) {
	root := filepath.Join("testdata", "conformance")
	var cases []c11WarmCase
	for _, family := range []string{"c11_q2", "c11_q345", "c11_q14_corrected", "c11_q3_repair_admin"} {
		dir := filepath.Join(root, family)
		data := c12Read(t, filepath.Join(dir, "provenance.json"))
		seen := map[string]bool{}
		switch family {
		case "c11_q2":
			var m c11q2Manifest
			if err := json.Unmarshal(data, &m); err != nil {
				t.Fatal(err)
			}
			for _, c := range m.Cases {
				key := c.API + "/" + c.Class
				if seen[key] {
					continue
				}
				seen[key] = true
				prefix := "project/" + c.API + "/"
				files := map[string]string{}
				for n, h := range m.Files {
					if strings.HasPrefix(n, prefix) {
						files[strings.TrimPrefix(n, prefix)] = h
					}
				}
				method := c.Method
				if c.Arm == "admin" {
					method = "baseline"
				}
				cases = append(cases, c11WarmCase{family + "/" + key, filepath.Join(dir, "project", c.API), c.Class, method, c.Arm, files})
			}
		case "c11_q345":
			var m c11q345Manifest
			if err := json.Unmarshal(data, &m); err != nil {
				t.Fatal(err)
			}
			for _, c := range m.Cases {
				key := c.Route + "/" + c.Class
				if seen[key] {
					continue
				}
				seen[key] = true
				prefix := "project/" + c.Route + "/"
				files := map[string]string{}
				for n, h := range m.Files {
					if strings.HasPrefix(n, prefix) {
						files[strings.TrimPrefix(n, prefix)] = h
					}
				}
				method := c.Method
				if c.Arm == "admin" {
					method = "baseline"
				}
				cases = append(cases, c11WarmCase{family + "/" + key, filepath.Join(dir, "project", c.Route), c.Class, method, c.Arm, files})
			}
		case "c11_q14_corrected":
			var m c11Q14Manifest
			if err := json.Unmarshal(data, &m); err != nil {
				t.Fatal(err)
			}
			for _, c := range m.Cases {
				key := c.API + "/" + c.Class
				if seen[key] {
					continue
				}
				seen[key] = true
				method := c.Method
				if c.Arm == "admin" {
					method = "baseline"
				}
				cases = append(cases, c11WarmCase{family + "/" + key, filepath.Join(dir, "project", c.API), c.Class, method, c.Arm, c.Files})
			}
		case "c11_q3_repair_admin":
			var m c11Q3Manifest
			if err := json.Unmarshal(data, &m); err != nil {
				t.Fatal(err)
			}
			for _, c := range m.Cases {
				cases = append(cases, c11WarmCase{family + "/" + c.API, filepath.Join(dir, "project", c.API), c.Class, "baseline", "admin", c.Files})
			}
		}
	}
	if len(cases) != 22 {
		t.Fatalf("warm source inventory changed: %d", len(cases))
	}
	restore := apextest.EnableDiskCacheForTesting()
	defer restore()
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			c12CopyProject(t, c.source, dir, c.files)
			index := c12LoadIndex(t, dir)
			apextest.InvalidateRuntimeCaches()
			options := apextest.Options{SelectedClasses: []string{c.class}, SelectedMethod: c.method, Parallelism: 1, PerfCounters: true}
			cold := apextest.Run(index, options)
			apextest.ResetPerfCounters()
			warm := apextest.Run(index, options)
			if len(cold.Suites) != 1 || len(cold.Suites[0].Cases) != 1 || len(warm.Suites) != 1 || len(warm.Suites[0].Cases) != 1 {
				t.Fatal("selected warm route missing")
			}
			a, b := cold.Suites[0].Cases[0], warm.Suites[0].Cases[0]
			if a.Status != b.Status || (a.Problem == nil) != (b.Problem == nil) ||
				(a.Problem != nil && a.Problem.Message != b.Problem.Message) {
				t.Fatalf("cold/warm prerequisite changed: %s / %s", c11Diagnostic(a), c11Diagnostic(b))
			}
			if apextest.SnapshotPerfCounters().Phases.MemoryCacheHits == 0 {
				t.Fatal("warm run did not hit runtime memory cache")
			}
		})
	}
}
