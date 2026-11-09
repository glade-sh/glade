package apextest

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/testreport"
)

func TestJSONGeneratorClosedNamedNativeConformance(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "json_generator_closed")
	read := func(name string) [][]string {
		f, err := os.Open(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		r := csv.NewReader(f)
		r.Comma = '\t'
		rows, err := r.ReadAll()
		if err != nil {
			t.Fatal(err)
		}
		return rows
	}
	bodies := map[string]string{}
	for _, r := range read("cases.tsv")[1:] {
		bodies[r[0]] = r[1]
	}
	rows := read("org.tsv")[1:]
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			expected := map[string]string{}
			var source strings.Builder
			source.WriteString("@IsTest private class ClosedGeneratorNativeTest {\n")
			for _, r := range rows {
				if r[0] != api || r[1] != "isTest" {
					continue
				}
				id, native := r[2], r[3]
				body, ok := bodies[id]
				if !ok {
					t.Fatal("missing native source")
				}
				expected[id] = native
				var script string
				if strings.HasPrefix(native, "uncaught|") {
					script = "try { " + body + " } catch(Exception e) { System.assert(false,'caught native fatal error'); } System.assert(false,'native error must escape');"
				} else {
					escaped := strings.NewReplacer("\\", "\\\\", "'", "\\'", "\n", "\\n").Replace(native)
					script = "String observed='accepted'; try { " + body + " } catch(Exception e) { observed='caught|'+e.getTypeName()+'|'+e.getMessage(); } System.assertEquals('" + escaped + "',observed);"
				}
				fmt.Fprintf(&source, "@IsTest static void %s() { %s }\n", id, script)
			}
			source.WriteString("}\n")
			if len(expected) != 25 {
				t.Fatalf("native rows=%d, want 25", len(expected))
			}
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			path := filepath.Join(root, "force-app/main/default/classes/ClosedGeneratorNativeTest.cls")
			writeFile(t, path, source.String())
			writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native test sources rejected: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			for _, warm := range []bool{false, true} {
				run := Run(index, Options{SelectedClasses: []string{"ClosedGeneratorNativeTest"}, Parallelism: 1})
				seen := map[string]bool{}
				for _, suite := range run.Suites {
					for _, c := range suite.Cases {
						native, ok := expected[c.MethodName]
						if !ok {
							t.Fatalf("unexpected test %s", c.MethodName)
						}
						seen[c.MethodName] = true
						if strings.HasPrefix(native, "uncaught|") {
							if c.Problem == nil || c.Problem.Type != "System.UnexpectedException" || !strings.HasPrefix(c.Problem.Message, "Salesforce System Error") {
								t.Errorf("warm=%v %s native uncaught error; local=%+v", warm, c.MethodName, c)
							}
						} else if c.Status != testreport.StatusPass {
							t.Errorf("warm=%v %s native %s; local=%+v", warm, c.MethodName, native, c)
						}
					}
				}
				if len(seen) != len(expected) {
					t.Fatalf("warm=%v named cases=%d, want %d", warm, len(seen), len(expected))
				}
			}
		})
	}
}
