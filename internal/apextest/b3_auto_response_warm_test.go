package apextest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestB3AutoResponseWarmProjectRoute(t *testing.T) {
	const fixture = "testdata/conformance/b3_auto_response"
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			if err := os.CopyFS(root, os.DirFS(filepath.Join(fixture, "project"))); err != nil {
				t.Fatal(err)
			}
			err := filepath.Walk(filepath.Join(root, "force-app/main/default/classes"), func(path string, info os.FileInfo, e error) error {
				if e != nil || info.IsDir() || !strings.HasSuffix(path, ".cls-meta.xml") {
					return e
				}
				data, e := os.ReadFile(path)
				if e != nil {
					return e
				}
				writeFile(t, path, strings.ReplaceAll(strings.ReplaceAll(string(data), "62.0", api), "67.0", api))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			raw, err := os.ReadFile(filepath.Join(fixture, "active", "api"+strings.TrimSuffix(api, ".0"), "istest", "org.tsv"))
			if err != nil {
				t.Fatal(err)
			}
			var observations []string
			for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n")[1:] {
				key, value, ok := strings.Cut(line, "\t")
				if !ok {
					t.Fatal(line)
				}
				observations = append(observations, key+"="+value)
			}
			if len(observations) != 3 {
				t.Fatal("incomplete native capture")
			}
			code := `System.assertEquals('` + strings.Join(observations, "|") + `',B3RuleProbe.run());`
			cls := filepath.Join(root, "force-app/main/default/classes/B3WarmAssertion.cls")
			writeFile(t, cls, `@IsTest private class B3WarmAssertion { @IsTest static void check() { `+code+` } }`)
			writeFile(t, cls+"-meta.xml", fmt.Sprintf(`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><status>Active</status></ApexClass>`, api))
			index := loadTestIndex(t, root)
			opts := Options{SelectedClasses: []string{"B3WarmAssertion"}, NoDiskCache: true, Parallelism: 1, PerfCounters: true}
			first := Run(index, opts)
			if first.Summary().Passed != 1 {
				t.Fatalf("cold: %+v", first)
			}
			ResetPerfCounters()
			warm := Run(index, opts)
			if warm.Summary().Passed != 1 {
				t.Fatalf("warm: %+v", warm)
			}
			if p := SnapshotPerfCounters().Phases; p.MemoryCacheHits != 1 || p.CacheMisses != 0 {
				t.Fatalf("warm cache: %+v", p)
			}
		})
	}
}
