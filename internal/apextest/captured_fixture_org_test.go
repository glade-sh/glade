package apextest

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

// Replay only assignments recorded by a fixture's native capture.
func capturedFixtureOrg(t *testing.T, index typesys.Index, api string) storage.OrgState {
	t.Helper()
	org := orgFromIndex(index)
	org.APIVersion = api
	p, err := project.Load(index.Project.Root)
	if err != nil {
		t.Fatal(err)
	}
	if err := applyConfiguredTestPermissions(&org, p); err != nil {
		t.Fatal(err)
	}
	return org
}

// writeCapturedAssignments configures the permission sets that a fixture's
// native capture assigned to the running user, so Run replays them.
func writeCapturedAssignments(t *testing.T, root string, names ...string) {
	t.Helper()
	path := filepath.Join(root, "glade.yml")
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	config := string(existing)
	if strings.Contains(config, "test:") {
		t.Fatalf("%s already configures test settings", path)
	}
	if config != "" && !strings.HasSuffix(config, "\n") {
		config += "\n"
	}
	writeFile(t, path, config+"test:\n  permissionSets: ["+strings.Join(names, ", ")+"]\n")
}

// capturedPermissionSetAssignmentFixtures names the fixtures whose native
// capture assigned the project's permission sets to the running user.
var capturedPermissionSetAssignmentFixtures = strings.Fields(`
	admin_field_grant_sources
	admin_optional_field_api_matrix
	admin_optional_field_explicit_mode
	blank_text_formula
	configured_permissions
	constructor_field_catalog
	custom_metadata_user_mode_columns
	deleted_lookup
	ext_upsert_capture
	flow_assign_routes
	flow_entry_matrix
	flow_lookup_no_match_id_native
	formula_br_function
	query_temporal_routes
	sobject_constructor_values
	sobject_json2
	sobject_json2_decoders
	sobject_json2_namespace
	sobject_json3
	soql_quoted_numeric
	stub_object_collision
	trigger_record_formulas
	url_field_limit
`)

// replayRecordedPermissionSetAssignments assigns every permission set in the
// copied project's permissionDir for a fixture whose native capture assigned
// them. Fixtures without that record keep their own setup.
func replayRecordedPermissionSetAssignments(t *testing.T, fixture, root, permissionDir string) {
	t.Helper()
	if !slices.Contains(capturedPermissionSetAssignmentFixtures, filepath.Base(fixture)) {
		return
	}
	entries, err := os.ReadDir(filepath.Join(root, permissionDir))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		if name, ok := strings.CutSuffix(entry.Name(), ".permissionset-meta.xml"); ok {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		t.Fatalf("%s records an assignment but the project has no permission set", fixture)
	}
	writeCapturedAssignments(t, root, names...)
}

// assignCapturedPermissionSets assigns named project permission sets to the
// default administrator of org, for routes that load a fixture in place.
func assignCapturedPermissionSets(t *testing.T, org *storage.OrgState, names ...string) {
	t.Helper()
	var p project.Project
	p.Test.PermissionSets = names
	if err := applyConfiguredTestPermissions(org, p); err != nil {
		t.Fatal(err)
	}
}

// writeHarnessAccessPermissionSet writes the permission set a capture harness
// generated and assigned to the running user: read and edit on every listed
// field, full object CRUD, and the harness's view/modify-all flags. Exported
// fixtures normalize the harness's per-API token names; the grant follows the
// same mapping. Callers name the harness source that defines the set.
func writeHarnessAccessPermissionSet(t *testing.T, root, name, object string, fields []string, viewModifyAll bool) {
	t.Helper()
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?><PermissionSet xmlns="http://soap.sforce.com/2006/04/metadata">`)
	for _, field := range fields {
		b.WriteString("<fieldPermissions><editable>true</editable><field>" + object + "." + field + "</field><readable>true</readable></fieldPermissions>")
	}
	all := strconv.FormatBool(viewModifyAll)
	b.WriteString("<label>" + name + "</label><objectPermissions><allowCreate>true</allowCreate><allowDelete>true</allowDelete><allowEdit>true</allowEdit><allowRead>true</allowRead><modifyAllRecords>" + all + "</modifyAllRecords><object>" + object + "</object><viewAllRecords>" + all + "</viewAllRecords></objectPermissions></PermissionSet>")
	writeFile(t, filepath.Join(root, "force-app/main/default/permissionsets", name+".permissionset-meta.xml"), b.String())
}

// customFieldNames lists the <fields><fullName> entries of a source-format
// .object file, in file order.
func customFieldNames(t *testing.T, objectXML []byte) []string {
	t.Helper()
	var object struct {
		Fields []struct {
			FullName string `xml:"fullName"`
		} `xml:"fields"`
	}
	if err := xml.Unmarshal(objectXML, &object); err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, field := range object.Fields {
		names = append(names, field.FullName)
	}
	return names
}

// configuredTestPermissionSets returns the project's glade.yml
// test.permissionSets, such as a replayed capture assignment.
func configuredTestPermissionSets(t *testing.T, root string) []string {
	t.Helper()
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	return p.Test.PermissionSets
}
