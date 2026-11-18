package apextest

import (
	"encoding/csv"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// Native rows: the administrator's explicit Database.insert(record,
// AccessLevel.USER_MODE) from a class at API 61.0 or 62.0, swallowed by a
// try/catch, for an object whose one optional field has no grant. The insert
// is denied on the field at both class versions, at test-class versions 62.0
// and 67.0, for an unnamespaced object and for the project's own namespace. A
// permission set that @TestSetup assigns does not help. A permission set
// assigned to the user before the run (glade.yml test.permissionSets) or a
// field grant on the Admin profile does.
func TestUserModeLegacyAdminGateNativeConformance(t *testing.T) {
	family := filepath.Join("testdata/conformance", "usermode_legacy_admin_gate")
	raw, err := os.Open(filepath.Join(family, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	reader := csv.NewReader(raw)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	reader.FieldsPerRecord = 6
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 65 || strings.Join(rows[0], ",") != "org,grant,test_api,class_api,repeat,observation" {
		t.Fatalf("incomplete native rows: %d", len(rows))
	}
	overlays := map[string]string{"preassigned-permset": "overlay-permset", "profile": "overlay-profile"}
	indexes := map[string]typesys.Index{}
	for _, org := range []string{"plain", "nsp"} {
		for _, overlay := range []string{"", "overlay-permset", "overlay-profile"} {
			root := t.TempDir()
			copyGateTree(t, filepath.Join(family, org), root)
			if overlay != "" {
				copyGateTree(t, filepath.Join(family, overlay), root)
			}
			indexes[org+"/"+overlay] = loadTestIndex(t, root)
		}
	}
	for i, row := range rows[1:] {
		org, grant, testAPI, classAPI, repeat, want := row[0], row[1], row[2], row[3], row[4], row[5]
		line := i + 2
		t.Run(org+"/"+grant+"/test"+testAPI+"/class"+classAPI+"/r"+repeat, func(t *testing.T) {
			index := indexes[org+"/"+overlays[grant]]
			class := "CxT0_" + strings.TrimSuffix(testAPI, ".0")
			if grant == "testsetup-permset" {
				class = "CxT1_" + strings.TrimSuffix(testAPI, ".0")
			}
			method := "m" + strings.TrimSuffix(classAPI, ".0")
			run := Run(index, Options{SelectedClasses: []string{class}, SelectedMethod: method, NoDiskCache: true, Parallelism: 1, LimitMode: vm.LimitModeStrict})
			if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
				t.Fatalf("missing runner observation %+v", run.Summary())
			}
			_, got, _ := strings.Cut(run.Suites[0].Cases[0].Problem.Message, " => ")
			if strings.TrimSpace(got) != strings.TrimSpace(want) {
				t.Fatalf("NATIVE %s.%s usermode_legacy_admin_gate/org.tsv:%d istest %s\nnative=%q\nglade =%q", class, method, line, classAPI, want, strings.TrimSpace(got))
			}
		})
	}
}

func copyGateTree(t *testing.T, from, to string) {
	t.Helper()
	err := filepath.WalkDir(from, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(from, path)
		if err != nil {
			return err
		}
		writeFile(t, filepath.Join(to, rel), string(body))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
