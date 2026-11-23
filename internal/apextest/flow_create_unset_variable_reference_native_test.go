package apextest

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// NATIVE rows: testdata/conformance/flow_create_unset_variable_reference/org.tsv.
// A record-triggered after-save Flow creates a PricebookEntry from a Pricebook2
// record variable that no loop iteration filled. In a test the lookup found no
// standard pricebook (with or without Test.getStandardPricebookId()), so the
// variable's Id is null and the Product2 insert fails with a catchable
// DmlException REQUIRED_FIELD_MISSING that carries the Flow's text, at API 62.0
// and 67.0.
func TestFlowCreateUnsetVariableReferenceNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/flow_create_unset_variable_reference"
	read := func(path string) string {
		t.Helper()
		body, err := os.ReadFile(filepath.Join(fixture, path))
		if err != nil {
			t.Fatal(err)
		}
		return string(body)
	}
	link := regexp.MustCompile(`<a href='[^']*'>SOAP API Developer Guide</a>`)
	normalize := func(text string) string {
		text = strings.ReplaceAll(text, "&#124;", "|")
		text = strings.ReplaceAll(text, "<a href='<org-url> API Developer Guide</a>", "SOAP API Developer Guide")
		return link.ReplaceAllString(text, "SOAP API Developer Guide")
	}
	wantByAPI := map[string]string{}
	for line, row := range strings.Split(strings.TrimSpace(read("org.tsv")), "\n")[1:] {
		cols := strings.Split(row, "\t")
		if len(cols) < 9 || cols[2] != "istest" {
			continue
		}
		if cols[0] != "probe" || cols[3] != "true" || cols[4] != "Failed" {
			t.Fatalf("invalid native row %d: %s", line+2, row)
		}
		wantByAPI[cols[1]] = normalize(cols[5])
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			want, ok := wantByAPI[api]
			if !ok {
				t.Fatalf("no native row for %s", api)
			}
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			err := filepath.WalkDir(filepath.Join(fixture, "project/force-app"), func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				name, err := filepath.Rel(filepath.Join(fixture, "project"), path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, name), strings.ReplaceAll(read(filepath.Join("project", name)), "__API__", api))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			run := Run(index, Options{SelectedClasses: []string{"R5PbProbe"}, NoDiskCache: true, Parallelism: 1})
			got := map[string]string{}
			for _, suite := range run.Suites {
				for _, c := range suite.Cases {
					if c.Problem != nil {
						got[c.Name] = c.Problem.Message
					}
				}
			}
			for _, method := range []string{"plain", "touched"} {
				message, ok := got[method]
				if !ok {
					message = got["R5PbProbe."+method]
				}
				if !strings.Contains(want, method+"=Fail System.AssertException: Assertion Failed: ") {
					t.Fatalf("native row has no %s outcome: %q", method, want)
				}
				expected := "R5|pb|" + strings.SplitN(strings.SplitN(want, method+"=Fail System.AssertException: Assertion Failed: R5|pb|", 2)[1], "; touched=", 2)[0]
				if !strings.Contains(normalize(message), expected) {
					t.Errorf("NATIVE flow_create_unset_variable_reference/org.tsv %s istest %s: want %q; Glade %q (cases %v)", api, method, expected, message, got)
				}
			}
		})
	}
}
