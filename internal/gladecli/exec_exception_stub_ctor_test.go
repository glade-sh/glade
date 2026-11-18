package gladecli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
)

// Linked platform exception stubs have empty-body constructors. Every
// constructor shape must keep its message and cause under a project.
func TestRunExecWithProjectAppliesPlatformExceptionConstructorArgs(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
	for _, typeName := range []string{"Apex.EmptyStackException", "AsyncException", "BigObjectException", "IllegalArgumentException"} {
		t.Run(typeName, func(t *testing.T) {
			source := `Exception root = new DmlException('root cause');
` + typeName + ` plain = new ` + typeName + `();
System.assertEquals(null, plain.getCause());
` + typeName + ` byMessage = new ` + typeName + `('msg');
System.assertEquals('msg', byMessage.getMessage());
System.assertEquals(null, byMessage.getCause());
` + typeName + ` byCause = new ` + typeName + `(root);
System.assert(byCause.getCause() === root, 'cause only');
` + typeName + ` both = new ` + typeName + `('wrapped', root);
System.assertEquals('wrapped', both.getMessage());
System.assert(both.getCause() === root, 'message and cause');
`
			var stdout, stderr bytes.Buffer
			code := Run(context.Background(), []string{"exec", "--project", root, source}, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("%s: code=%d stdout=%q stderr=%q", typeName, code, stdout.String(), stderr.String())
			}
			if strings.Contains(stdout.String()+stderr.String(), "Script-thrown exception") {
				t.Fatalf("%s dropped constructor args: %q %q", typeName, stdout.String(), stderr.String())
			}
		})
	}
}
