package gladecli

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"
)

func TestRunExecProjectInitializesMetadataDeployResultMessages(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{
		"exec",
		"--project", root,
		"--json",
		`String serialized = JSON.serialize(new Metadata.DeployResult()); System.assert(serialized.contains('"messages":[]'), serialized);`,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exec failed code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}
