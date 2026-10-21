package gladecli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigTestPermissionsSurface(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, filepath.Join(root, "glade.yml"), "test:\n  permissionSets: [Synthetic_Access]\n  permissionSetGroups: [Synthetic_Group]\n")
	writeTestFile(t, filepath.Join(root, "force-app/main/default/permissionsets/Synthetic_Access.permissionset-meta.xml"), `<PermissionSet><label>Synthetic Access</label></PermissionSet>`)
	writeTestFile(t, filepath.Join(root, "force-app/main/default/permissionsetgroups/Synthetic_Group.permissionsetgroup-meta.xml"), `<PermissionSetGroup><label>Synthetic Group</label><permissionSets>Synthetic_Access</permissionSets></PermissionSetGroup>`)
	for _, args := range [][]string{{"config", "validate", "--project", root}, {"config", "show", "--project", root}, {"config", "show", "--project", root, "--json"}} {
		var out, stderr bytes.Buffer
		if exit := Run(context.Background(), args, &out, &stderr); exit != 0 {
			t.Fatalf("%v exit=%d: %s", args, exit, stderr.String())
		}
		if args[1] == "show" && (!strings.Contains(out.String(), "Synthetic_Access") || !strings.Contains(out.String(), "Synthetic_Group")) {
			t.Fatalf("show omits test assignments: %s", out.String())
		}
	}
	for _, key := range []string{"permissionSets", "permissionSetGroups"} {
		writeTestFile(t, filepath.Join(root, "glade.yml"), "test:\n  "+key+": [Synthetic_NotPresent]\n")
		for _, command := range []string{"config", "test"} {
			args := []string{command, "--project", root}
			if command == "config" {
				args = []string{"config", "validate", "--project", root}
			}
			var out, stderr bytes.Buffer
			if exit := Run(context.Background(), args, &out, &stderr); exit == 0 {
				t.Fatalf("%s accepted missing metadata", command)
			}
			if text := out.String() + stderr.String(); !strings.Contains(text, "Synthetic_NotPresent") || !strings.Contains(text, key) {
				t.Fatalf("missing named config error: %s", text)
			}
		}
	}
}
