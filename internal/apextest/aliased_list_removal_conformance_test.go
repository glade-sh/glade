package apextest

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestAliasedListRemovalOrgConformance(t *testing.T) {
	const fixture = "testdata/conformance/aliased_list_removal"
	var data struct {
		Route string
		Rows  []struct {
			ID, API, Class, Method string
			Native                 *struct{ Type, Message string }
		}
	}
	raw, err := os.ReadFile(filepath.Join(fixture, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if data.Route != "istest" || len(data.Rows) != 8 {
		t.Fatal("incomplete native list-removal table")
	}
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	for _, name := range []string{"AliasedRemoval62", "AliasedRemoval67"} {
		api := strings.TrimPrefix(name, "AliasedRemoval")
		var capture struct {
			SourceAPI    string
			SourceSHA256 map[string]string
		}
		raw, err := os.ReadFile(filepath.Join(fixture, "native/api"+api, "RUN-FACTS.txt"))
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw, &capture); err != nil {
			t.Fatal(err)
		}
		if capture.SourceAPI != api+".0" {
			t.Fatal("native source API pin changed")
		}
		for _, suffix := range []string{".cls", ".cls-meta.xml"} {
			source, err := os.ReadFile(filepath.Join(fixture, "fixture/classes", name+suffix))
			if err != nil {
				t.Fatal(err)
			}
			if got := fmt.Sprintf("%x", sha256.Sum256(source)); got != capture.SourceSHA256[name+suffix] {
				t.Fatalf("captured source changed: %s", name+suffix)
			}
			writeFile(t, filepath.Join(root, "force-app/main/default/classes", name+suffix), string(source))
		}
	}
	index := loadTestIndex(t, root)
	analysis := sema.Analyze(index)
	if index.HasErrors() || analysis.HasErrors() {
		t.Fatalf("captured source rejected: %v %v", index.Diagnostics, analysis.Diagnostics)
	}
	cases := Discover(index, Options{SelectedClasses: []string{"AliasedRemoval62", "AliasedRemoval67"}})
	if len(cases) != len(data.Rows) {
		t.Fatalf("native methods=%d; local methods=%d", len(data.Rows), len(cases))
	}
	methods, methodErrors := compileTestMethods(cases)
	programs, programErrors := compileTestInvokePrograms(cases)
	org := orgFromIndex(index)
	runner := newConformanceRunner(t, index, conformanceRunnerOptions{Org: &org, LinkProject: true})
	for _, row := range data.Rows {
		t.Run(row.ID, func(t *testing.T) {
			if row.Native == nil || row.Native.Type == "" || row.Native.Message == "" {
				t.Fatal("missing native observation")
			}
			nativeTSV, err := os.ReadFile(filepath.Join(fixture, "native/api"+strings.TrimSuffix(row.API, ".0"), "org.tsv"))
			if err != nil {
				t.Fatal(err)
			}
			nativeLine := strings.Join([]string{row.ID, row.Class, row.Method, "Fail", row.Native.Type, row.Native.Message}, "\t")
			if !strings.Contains("\n"+string(nativeTSV), "\n"+nativeLine+"\n") {
				t.Fatal("case table differs from native row")
			}
			for _, c := range cases {
				if c.ClassName != row.Class || c.MethodName != row.Method {
					continue
				}
				key := testCaseKey(c)
				if methods[key].APIVersion != row.API {
					t.Fatalf("source API=%s; native API=%s", methods[key].APIVersion, row.API)
				}
				if methodErrors[key] != nil || programErrors[key] != nil {
					t.Fatalf("compilation: %v %v", methodErrors[key], programErrors[key])
				}
				machine := runner.newMachine()
				machine.EnableTestContext()
				if err := machine.RegisterMethod(methods[key]); err != nil {
					t.Fatal(err)
				}
				_, execErr := machine.ExecuteInClass(programs[key], c.ClassName)
				var fault *vm.RuntimeError
				if !errors.As(execErr, &fault) {
					t.Fatalf("missing terminal native observation: %v", execErr)
				}
				kind := fault.Type
				if !strings.HasPrefix(kind, "System.") {
					kind = "System." + kind
				}
				if kind != row.Native.Type || fault.ExceptionMessage() != row.Native.Message {
					t.Fatalf("native=%s: %s; local=%s: %s", row.Native.Type, row.Native.Message, kind, fault.ExceptionMessage())
				}
				return
			}
			t.Fatalf("captured method not discovered: %s.%s", row.Class, row.Method)
		})
	}
}
