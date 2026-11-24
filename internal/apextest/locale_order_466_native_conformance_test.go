package apextest

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

func TestLocaleOrder466NativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/locale_order_466"
	read := func(name string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(fixture, name))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	lines := strings.Split(strings.TrimSpace(read("org.tsv")), "\n")
	if len(lines) != 25 || lines[0] != "locale\tapi\troute\torder" {
		t.Fatal("incomplete native capture")
	}
	rows := make(map[string]string)
	for _, line := range lines[1:] {
		c := strings.Split(line, "\t")
		if len(c) != 4 {
			t.Fatal("malformed native row")
		}
		key := c[0] + "/" + c[1] + "/" + c[2]
		if _, exists := rows[key]; exists {
			t.Fatalf("duplicate row %s", key)
		}
		rows[key] = c[3]
	}
	locales := []string{"de_DE", "sv_SE", "ja_JP", "fr_FR", "es_ES", "en_US"}
	payload := regexp.MustCompile(`(de_DE|sv_SE|ja_JP|fr_FR|es_ES|en_US):([0-9,]+)`)
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			name := "LocaleCapture" + api[:2]
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			for _, suffix := range []string{".cls", ".cls-meta.xml"} {
				writeFile(t, filepath.Join(root, "force-app/main/default/classes", name+suffix), read(name+suffix))
			}
			index := loadTestIndex(t, root)
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{Org: &org, LinkProject: true})
			source := read("seed.apex") + "\n" + read("query.apex")
			program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			anonymous := map[string]string{}
			for _, locale := range locales {
				locale := locale
				result, err := runner.execute(program, func(machine *vm.VM) {
					machine.SetCurrentUser(storage.Record{ID: storage.ID("005-local-locale-user"), Object: "User", Fields: map[string]storage.Value{"LocaleSidKey": storage.StringValue(locale), "LanguageLocaleKey": storage.StringValue("en_US")}})
				})
				if err != nil {
					t.Fatalf("anonymous %s failed: %v", locale, err)
				}
				for _, line := range result.Debug {
					if m := payload.FindStringSubmatch(line); len(m) == 3 && m[1] == locale {
						anonymous[locale] = m[2]
					}
				}
				if anonymous[locale] == "" {
					t.Fatalf("anonymous %s has no observation", locale)
				}
			}
			run := Run(index, Options{SelectedClasses: []string{name}, NoDiskCache: true, Parallelism: 1})
			if run.Summary().Total != 6 {
				t.Fatalf("test selection: %+v", run.Summary())
			}
			testRows := map[string]string{}
			for _, suite := range run.Suites {
				for _, test := range suite.Cases {
					if test.Problem == nil {
						t.Fatalf("test %s did not produce its intentional assertion", test.MethodName)
					}
					m := payload.FindStringSubmatch(test.Problem.Message)
					if len(m) != 3 {
						t.Fatalf("test %s did not produce an ordering observation (%s): %s", test.MethodName, test.Problem.Type, regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+`).ReplaceAllString(test.Problem.Message, "[redacted]"))
					}
					if test.MethodName != "t_"+m[1] {
						t.Fatalf("test locale mismatch: %s", test.MethodName)
					}
					testRows[m[1]] = m[2]
				}
			}
			for _, locale := range locales {
				for _, nativeRoute := range []string{"anonymous", "istest"} {
					key := locale + "/" + api + "/" + nativeRoute
					want, ok := rows[key]
					if !ok {
						t.Fatalf("missing native row %s", key)
					}
					for route, got := range map[string]string{"anonymous": anonymous[locale], "istest": testRows[locale]} {
						if got != want {
							t.Errorf("native %s / Glade %s: got %s want %s", key, route, got, want)
						}
					}
				}
			}
		})
	}
}
