package apextest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

// Replay the original capture PermissionSet metadata. Older FlowCore exports
// normalize transport object names; their retained grants use the same mapping.
// No field grant is inferred from the local schema or a successful observation.
func configureFlowFixtureFieldAccess(t *testing.T, root string) {
	t.Helper()
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	if len(p.PermissionSetFiles) != 0 {
		// Neighbour captures retain their complete permission sets in fixture.Files.
		for _, path := range p.PermissionSetFiles {
			names = append(names, metadataNameFromPath(path, ".permissionset-meta.xml", ".permissionset"))
		}
	} else {
		var core map[string]string
		var refs map[string]struct{ Name, XML string }
		for file, target := range map[string]any{"core.json": &core, "refs.json": &refs} {
			raw, err := os.ReadFile(filepath.Join("testdata/conformance/flow_capture_permissions", file))
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(raw, target); err != nil {
				t.Fatal(err)
			}
		}
		schema, err := schema.LoadProject(p)
		if err != nil {
			t.Fatal(err)
		}
		var chunks []string
		for _, object := range schema.Objects {
			if !strings.HasSuffix(object.Name, "__c") || object.CustomSettingsType != "" {
				continue
			}
			if fields, ok := core[object.Name]; ok {
				chunks = append(chunks, fields)
			} else if grant, ok := refs[object.Name]; ok {
				writeFile(t, filepath.Join(root, "force-app/main/default/permissionsets", grant.Name+".permissionset-meta.xml"), grant.XML)
				names = append(names, grant.Name)
			} else {
				// Other callers of the shared metadata factory can intentionally
				// have no field access (the native catalog-copy denial, for example).
				// Only the retained capture grants above are replayed.
				continue
			}
		}
		if len(chunks) != 0 {
			sort.Strings(chunks)
			writeFile(t, filepath.Join(root, "force-app/main/default/permissionsets/FlowCoreCaptureAccess.permissionset-meta.xml"),
				`<PermissionSet xmlns="http://soap.sforce.com/2006/04/metadata"><label>FlowCore capture access</label>`+strings.Join(chunks, "")+`</PermissionSet>`)
			names = append(names, "FlowCoreCaptureAccess")
		}
	}
	sort.Strings(names)
	writeFile(t, filepath.Join(root, "glade.yml"), "test:\n  permissionSets: ["+strings.Join(names, ", ")+"]\n")
}

// loadProjectForTest reads the project configuration (glade.yml) under root
// for applyConfiguredTestPermissions; the typesys index carries only the
// package layout.
func loadProjectForTest(t *testing.T, root string) project.Project {
	t.Helper()
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// Check setup independently of business observations: native capture permission
// assignments must survive fixture loading and runtime-template construction.
func assertCapturedPermissionAssignments(t *testing.T, org *storage.OrgState, names ...string) {
	t.Helper()
	var source strings.Builder
	for _, name := range names {
		fmt.Fprintf(&source, "System.assertEquals(1,[SELECT COUNT() FROM PermissionSetAssignment WHERE AssigneeId=:UserInfo.getUserId() AND PermissionSet.Name='%s'], 'captured assignment %s');\n", name, name)
	}
	machine := vm.New(nil)
	machine.SetOrg(org)
	program, err := vm.CompileAnonymousWithOptions(source.String(), vm.CompileOptions{APIVersion: org.APIVersion})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

// Load the exact permission metadata retained with the owned native capture.
// Assignment is a separate setup transaction, before either observation route.
func configureCapturedFixturePermissions(t *testing.T, root, fixture string, names ...string) {
	t.Helper()
	for _, name := range names {
		file := name + ".permissionset-meta.xml"
		raw, err := os.ReadFile(filepath.Join(fixture, "permissionsets", file))
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(root, "force-app/main/default/permissionsets", file), string(raw))
	}
	writeFile(t, filepath.Join(root, "glade.yml"), "test:\n  permissionSets: ["+strings.Join(names, ", ")+"]\n")
}
