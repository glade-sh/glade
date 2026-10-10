package gladecli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunExecWithProjectResolvesTransientDeclarationTypes(t *testing.T) {
	for _, apiVersion := range []string{"62.0", "67.0"} {
		t.Run(apiVersion, func(t *testing.T) {
			root := t.TempDir()
			t.Chdir(root)
			writeProjectWithWidgetField(t, root, "Label__c")
			writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+apiVersion+`"}`)
			// These project declarations must retain their top-level rules when
			// the anonymous declaration is checked against the project index.
			writeTestFile(t, filepath.Join(root, "force-app/main/default/classes/ExistingType.cls"), `public class ExistingType {
 public static Integer count() { return 1; }
 public class Nested {}
}`)
			for _, test := range []struct {
				name       string
				source     string
				wantReject bool
			}{
				{name: "projectType", source: `class Local { public ExistingType value; }`},
				{name: "projectSchema", source: `class Local { public Widget__c value; }`},
				// Unknown declaration types must still fail before execution.
				{name: "unknownType", source: `class Local { public UnknownType value; } System.assert(false, 'must not execute');`, wantReject: true},
			} {
				t.Run(test.name, func(t *testing.T) {
					var stdout, stderr bytes.Buffer
					code := Run(context.Background(), []string{"exec", "--project", root, "--json", test.source}, &stdout, &stderr)
					if test.wantReject {
						if code == 0 || !strings.Contains(stderr.String(), "GLADESEMA002") || !strings.Contains(stderr.String(), "UnknownType") {
							t.Fatalf("unknown type must fail before execution: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
						}
					} else if code != 0 {
						t.Fatalf("project declaration must resolve: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
					}
				})
			}
		})
	}
}
