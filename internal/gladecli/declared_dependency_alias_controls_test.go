package gladecli

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

// Native anonymous controls: ../apextest/testdata/conformance/
// declared_dependency_alias_capture/api{62,67}/org.tsv:2,6.
func TestDeclaredDependencyAliasAvailabilityAnonymous(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, alias := range []bool{false, true} {
			for _, present := range []bool{false, true} {
				t.Run(fmt.Sprintf("api%s/alias%t/present%t", api, alias, present), func(t *testing.T) {
					workspace := t.TempDir()
					root := filepath.Join(workspace, "consumer")
					aliases := ""
					if alias {
						aliases = `,"packageAliases":{"CedarLibrary":"0Ho000000000000AAA"}`
					}
					writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true,"dependencies":[{"package":"CedarLibrary"}]}]`+aliases+`}`)
					if present {
						supplier := filepath.Join(workspace, "supplier")
						writeTestFile(t, filepath.Join(supplier, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true,"package":"CedarLibrary"}]}`)
						path := filepath.Join(supplier, "force-app/main/default/classes/CedarLibrary.cls")
						writeTestFile(t, path, `public class CedarLibrary { public static Integer value(){ return 7; } }`)
						writeTestFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>")
					}
					var stdout, stderr bytes.Buffer
					code := Run(context.Background(), []string{"exec", "--project", root, "--json", `System.assertEquals(7,CedarLibrary.value());`}, &stdout, &stderr)
					output := stdout.String() + stderr.String()
					if present && code != 0 {
						t.Fatalf("present code=%d output=%s", code, output)
					}
					if !present && (code == 0 || !strings.Contains(output, "dependency_missing")) {
						t.Fatalf("absent code=%d output=%s", code, output)
					}
				})
			}
		}
	}
}
