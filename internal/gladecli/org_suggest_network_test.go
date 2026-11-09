package gladecli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestOrgSuggestNetworkField(t *testing.T) {
	root := t.TempDir()
	writeDoctorFixture(t, root, map[string]string{
		"sfdx-project.json":                `{"packageDirectories":[{"path":"force-app"}]}`,
		"force-app/lwc/example/example.js": `import FIELD from '@salesforce/schema/Network.Name';`,
	})
	for _, jsonOut := range []bool{false, true} {
		args := []string{"org", "suggest", "--project", root}
		if jsonOut {
			args = append(args, "--json")
		}
		var stdout, stderr bytes.Buffer
		if code := Run(context.Background(), args, &stdout, &stderr); code != 0 {
			t.Fatalf("json=%t: exit=%d stdout=%q stderr=%q", jsonOut, code, stdout.String(), stderr.String())
		}
		if jsonOut && !json.Valid(stdout.Bytes()) {
			t.Fatalf("invalid JSON: %s", stdout.String())
		}
		if !strings.Contains(stdout.String(), "NetworksEnabled") || strings.Contains(stdout.String(), "features: [Communities]") {
			t.Fatalf("json=%t: missing Network setting: %s", jsonOut, stdout.String())
		}
	}
}
