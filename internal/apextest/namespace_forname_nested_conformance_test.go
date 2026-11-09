package apextest

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

// Native rows: testdata/conformance/namespace_forname_nested/org.tsv. In a
// namespaced project Type.forName finds a nested class only when it is public
// or global, whichever spelling names it (forName(ns, 'Outer.Inner'),
// 'ns.Outer.Inner', a null namespace, a bare name or the 'ns.Outer$Inner'
// form). Private, default-access and @TestVisible nested classes are null at
// every class apiVersion, from anonymous Apex, an @IsTest class and the
// top-level type's own helper.
const namespaceForNameNestedDir = "testdata/conformance/namespace_forname_nested"

func TestNamespaceForNameNestedNativeConformance(t *testing.T) {
	read := func(rel string) string {
		body, err := os.ReadFile(filepath.Join(namespaceForNameNestedDir, rel))
		if err != nil {
			t.Fatal(err)
		}
		return string(body)
	}
	lines := strings.Split(strings.TrimSpace(read("org.tsv")), "\n")
	if lines[0] != "api\troute\tcase\tobserved" || len(lines) != 193 {
		t.Fatalf("org.tsv header/rows: %q, %d lines", lines[0], len(lines))
	}
	type key struct{ api, route string }
	groups := map[key][][2]string{}
	var order []key
	for _, line := range lines[1:] {
		c := strings.Split(line, "\t")
		if len(c) != 4 {
			t.Fatalf("bad row %q", line)
		}
		k := key{c[0], c[1]}
		if _, ok := groups[k]; !ok {
			order = append(order, k)
		}
		groups[k] = append(groups[k], [2]string{c[2], c[3]})
	}
	for _, k := range order {
		k := k
		t.Run(k.api+"_"+k.route, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace":"gladens004","sourceApiVersion":"`+k.api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			meta := "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>" + k.api + "</apiVersion><status>Active</status></ApexClass>"
			for _, name := range []string{"NsVis", "NsVisTest"} {
				writeFile(t, filepath.Join(root, "force-app/main/default/classes", name+".cls"), read("classes/"+name+".cls"))
				writeFile(t, filepath.Join(root, "force-app/main/default/classes", name+".cls-meta.xml"), meta)
			}
			index := loadTestIndex(t, root)
			var observed string
			if k.route == "anonymous" {
				program, err := vm.CompileAnonymousWithOptions(read("anon/a.apex"), vm.CompileOptions{APIVersion: k.api})
				if err != nil {
					t.Fatal(err)
				}
				org := orgFromIndex(index)
				org.APIVersion = k.api
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				result, err := runner.execute(program, func(m *vm.VM) { m.SetCurrentNamespace(org.Namespace) })
				if err != nil {
					t.Fatal(err)
				}
				observed = strings.Join(result.Debug, "\n")
			} else {
				run := Run(index, Options{SelectedClasses: []string{"NsVisTest"}, NoDiskCache: true, Parallelism: 1})
				for _, suite := range run.Suites {
					for _, c := range suite.Cases {
						if c.Problem != nil {
							observed += c.Problem.Message + "\n"
						}
					}
				}
				if observed == "" {
					t.Fatalf("no observation (%s) %+v", firstRunProblem(run), run.Summary())
				}
			}
			for _, row := range groups[k] {
				re := regexp.MustCompile(`(?m)OBS\|` + regexp.QuoteMeta(row[0]) + `\|[^|\n]*\|` + regexp.QuoteMeta(row[1]) + `$`)
				if !re.MatchString(observed) {
					got := regexp.MustCompile(`(?m)OBS\|` + regexp.QuoteMeta(row[0]) + `\|[^|\n]*\|[^\n]*`).FindString(observed)
					t.Errorf("%s %s %s: Glade %q; native %q", k.api, k.route, row[0], got, row[1])
				}
			}
		})
	}
}
