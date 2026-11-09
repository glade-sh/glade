package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStaticGetterReentryNative(t *testing.T) {
	const fixture = "testdata/conformance/static_getter_reentry"
	data, err := os.ReadFile(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 5 {
		t.Fatalf("expected four native rows, got %d", len(lines)-1)
	}
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	for _, name := range []string{"CaptureGetter62", "CaptureGetter67", "CaptureRecursion62", "CaptureRecursion67"} {
		for _, suffix := range []string{".cls", ".cls-meta.xml"} {
			b, err := os.ReadFile(filepath.Join(fixture, "project/classes", name+suffix))
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(root, "force-app/main/default/classes", name+suffix), string(b))
		}
	}
	index := loadTestIndex(t, root)
	for _, tc := range []struct {
		row           int
		class, method string
		fail          bool
	}{
		{1, "CaptureGetter62", "finiteReentry", false}, {2, "CaptureGetter67", "finiteReentry", false},
		{3, "CaptureRecursion62", "genuineRecursion", true}, {4, "CaptureRecursion67", "genuineRecursion", true},
	} {
		t.Run(tc.class, func(t *testing.T) {
			if !strings.Contains(lines[tc.row], map[bool]string{true: "System.LimitException: Maximum stack depth reached: 1001", false: "Pass; visits=2"}[tc.fail]) {
				t.Fatal("unexpected native row", lines[tc.row])
			}
			run := Run(index, Options{SelectedClasses: []string{tc.class}, SelectedMethod: tc.method, NoDiskCache: true, Parallelism: 1})
			if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
				t.Fatalf("missing test result: %+v", run.Summary())
			}
			result := run.Suites[0].Cases[0]
			if !tc.fail && result.Problem != nil {
				t.Fatalf("native passed, local failed: %+v", result.Problem)
			}
			if tc.fail && (result.Problem == nil || !strings.Contains(result.Problem.Message, "Maximum stack depth reached: 1001")) {
				t.Fatalf("native recursion reached stack limit, local result: %+v", result)
			}
		})
	}
}
