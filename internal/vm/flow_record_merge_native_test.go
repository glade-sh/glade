package vm_test

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// flow_record_merge holds native Custom Error text for merge fields of $Record
// Number, Currency, Percent, DateTime and Time fields (API 62.0 and 67.0) and
// of flow variables, constants and formulas. The anonymous and @IsTest native
// routes agree on every row; the project runs the @IsTest route.
func TestFlowRecordMergeNativeConformance(t *testing.T) {
	dir := filepath.Join("testdata", "flow_record_merge")
	file, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	native := map[string]string{}
	anonymous := map[string]string{}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	scanner.Scan()
	for scanner.Scan() {
		parts := strings.SplitN(scanner.Text(), "\t", 4)
		if len(parts) != 4 {
			t.Fatalf("bad row %q", scanner.Text())
		}
		key := parts[0] + "/" + parts[2]
		switch parts[1] {
		case "istest":
			native[key] = parts[3]
		case "anonymous":
			anonymous[key] = parts[3]
		}
	}
	if len(native) != 38 || len(anonymous) != 38 {
		t.Fatalf("native rows: istest %d anonymous %d, want 38 each", len(native), len(anonymous))
	}
	for key, want := range native {
		if anonymous[key] != want {
			t.Fatalf("native routes disagree for %s:\n istest: %s\n anonymous: %s", key, want, anonymous[key])
		}
	}
	root := t.TempDir()
	source := filepath.Join(dir, "project")
	err = filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		return os.WriteFile(target, body, 0644)
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(apextest.InvalidateRuntimeCaches)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	sch, err := schema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	report := apextest.Run(typesys.Build(p, sch), apextest.Options{NoDiskCache: true})
	seen := 0
	for _, suite := range report.Suites {
		for _, c := range suite.Cases {
			name := c.MethodName
			index := strings.Index(name, "api")
			if index < 0 {
				t.Fatalf("unexpected method %s", name)
			}
			key := name[index+3:] + ".0/" + name[:index]
			want, ok := native[key]
			if !ok {
				t.Fatalf("no native row for %s", key)
			}
			seen++
			got := ""
			if c.Problem != nil {
				got = strings.TrimPrefix(c.Problem.Message, "Assertion Failed: OBS ")
			}
			if got != want {
				t.Errorf("%s\n got: %s\nwant: %s", key, got, want)
			}
		}
	}
	if seen != 38 {
		t.Fatalf("ran %d methods, want 38", seen)
	}
}
