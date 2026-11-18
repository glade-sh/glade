package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStandardUserExtUpsertNativeConformance(t *testing.T) {
	dir := filepath.Join("testdata", "native-capture", "standard_ext_upsert_standard_user")
	body, err := os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	type key struct{ set, api string }
	rows := map[key]map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(body)), "\n")[1:] {
		f := strings.SplitN(line, "\t", 4)
		if len(f) != 4 {
			t.Fatal("invalid native row")
		}
		k := key{f[0], f[1]}
		if rows[k] == nil {
			rows[k] = map[string]string{}
		}
		rows[k][f[2]] = f[3]
	}
	if len(rows) != 4 {
		t.Fatalf("native groups=%d", len(rows))
	}
	for k, want := range rows {
		k, want := k, want
		t.Run(k.set+"/"+k.api, func(t *testing.T) {
			root := t.TempDir()
			err := filepath.Walk(filepath.Join(dir, "project"), func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return err
				}
				rel, _ := filepath.Rel(filepath.Join(dir, "project"), path)
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				s := strings.ReplaceAll(string(data), "67.0", k.api)
				if k.set == "granted" && strings.HasSuffix(rel, "ExtProbeTest.cls") {
					s = strings.Replace(s, "  insert u;", "  insert u;\n  PermissionSet ps=[SELECT Id FROM PermissionSet WHERE Name='ExtKey' LIMIT 1];\n  insert new PermissionSetAssignment(AssigneeId=u.Id,PermissionSetId=ps.Id);", 1)
				}
				writeFile(t, filepath.Join(root, rel), s)
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			run := Run(index, Options{SelectedClasses: []string{"ExtProbeTest"}, SelectedMethod: "extProbe", NoDiskCache: true, Parallelism: 1})
			if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
				t.Fatalf("missing observation: %+v", run.Summary())
			}
			_, obs, ok := strings.Cut(run.Suites[0].Cases[0].Problem.Message, "OBSERVED_EXT\n")
			if !ok {
				t.Fatalf("missing marker: %+v", run.Suites[0].Cases[0].Problem)
			}
			got := map[string]string{}
			for _, line := range strings.Split(obs, "\n") {
				name, val, ok := strings.Cut(line, "=")
				if ok {
					got[name] = val
				}
			}
			if len(want) != 36 {
				t.Fatalf("native rows=%d", len(want))
			}
			for name, v := range want {
				if got[name] != v {
					t.Errorf("%s: got %q want %q", name, got[name], v)
				}
			}
		})
	}
}
