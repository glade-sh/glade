package gladecli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
)

// Native anonymous receiver rows: receiver_split/org.tsv:70-71, 108-109.
// A class-method bound dot is covered by receiver_split/org.tsv:114-115.
func TestReceiverTriviaCLIRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			for _, tc := range []struct{ name, source, want string }{
				{"free", "String\n.valueOf('x');", "Variable does not exist: String"},
				{"bound", "System.\ndebug('x');", "Unexpected token 'debug'."},
			} {
				var out, errOut bytes.Buffer
				code := Run(context.Background(), []string{"exec", "--project", root, tc.source}, &out, &errOut)
				if code == 0 || !strings.Contains(errOut.String(), tc.want) {
					t.Errorf("%s: code=%d stdout=%q stderr=%q", tc.name, code, out.String(), errOut.String())
				}
			}
			testPath := filepath.Join(root, "force-app/main/default/classes/ReceiverRouteTest.cls")
			writeTestFile(t, testPath, "@IsTest private class ReceiverRouteTest { @IsTest static void run() { System.\ndebug('x'); } }")
			writeTestFile(t, testPath+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			var out, errOut bytes.Buffer
			code := Run(context.Background(), []string{"test", "--project", root, "--class", "ReceiverRouteTest", "--json", "--no-progress", "--no-cache", "--no-serve"}, &out, &errOut)
			if code == 0 || !strings.Contains(out.String()+errOut.String(), "Missing ';' at 'debug'") {
				t.Errorf("runner: code=%d stdout=%q stderr=%q", code, out.String(), errOut.String())
			}
		})
	}
}
