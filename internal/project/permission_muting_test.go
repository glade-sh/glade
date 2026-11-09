package project

import (
	"reflect"
	"testing"
)

func mutingSetXML(flags string) []byte {
	return []byte(`<MutingPermissionSet xmlns="http://soap.sforce.com/2006/04/metadata"><objectPermissions>` + flags + `<object>X__c</object></objectPermissions><label>L</label></MutingPermissionSet>`)
}

// Native deployment reports one error per muting set, the first violated rule
// in Read, Edit, Delete, View All order (permset_muting_partial C01, C03, X12,
// X13), with the captured sentence whether the dependencies are partly or
// wholly unmuted.
func TestMutingObjectProblemsUsesCapturedSentences(t *testing.T) {
	const (
		read    = "To mute Read, first mute Create, Edit, Delete, View All Records, and Modify All Records."
		edit    = "To mute Edit, first mute Delete and Modify All Records."
		del     = "To mute Delete, first mute Modify All Records."
		viewAll = "__MISSING LABEL__ PropertyFile - val View All Records not found in section Page_ObjectPermissions_Edit_Muting"
	)
	cases := map[string]struct {
		flags string
		want  []string
	}{
		"edit only":           {"<allowEdit>true</allowEdit>", []string{edit}},
		"delete only":         {"<allowDelete>true</allowDelete>", []string{del}},
		"read only":           {"<allowRead>true</allowRead>", []string{read}},
		"edit and delete":     {"<allowEdit>true</allowEdit><allowDelete>true</allowDelete>", []string{edit}},
		"edit and modify":     {"<allowEdit>true</allowEdit><modifyAllRecords>true</modifyAllRecords>", []string{edit}},
		"read create edit":    {"<allowRead>true</allowRead><allowCreate>true</allowCreate><allowEdit>true</allowEdit>", []string{read}},
		"read but view all":   {"<allowRead>true</allowRead><allowCreate>true</allowCreate><allowEdit>true</allowEdit><allowDelete>true</allowDelete><modifyAllRecords>true</modifyAllRecords>", []string{read}},
		"view all only":       {"<viewAllRecords>true</viewAllRecords>", []string{viewAll}},
		"delete and view all": {"<allowDelete>true</allowDelete><viewAllRecords>true</viewAllRecords>", []string{del}},
		"edit and view all":   {"<allowEdit>true</allowEdit><viewAllRecords>true</viewAllRecords>", []string{edit}},
		"modify all only":     {"<modifyAllRecords>true</modifyAllRecords>", nil},
		"create only":         {"<allowCreate>true</allowCreate>", nil},
		"edit delete modify":  {"<allowEdit>true</allowEdit><allowDelete>true</allowDelete><modifyAllRecords>true</modifyAllRecords>", nil},
		"delete modify":       {"<allowDelete>true</allowDelete><modifyAllRecords>true</modifyAllRecords>", nil},
		"view all modify":     {"<viewAllRecords>true</viewAllRecords><modifyAllRecords>true</modifyAllRecords>", nil},
		"nothing muted":       {"<allowRead>false</allowRead>", nil},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := mutingObjectProblems(mutingSetXML(tc.flags)); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}

// Two violating objects in one set still give one error (C10, X10b, X10c).
func TestMutingObjectProblemsOneErrorPerSet(t *testing.T) {
	data := []byte(`<MutingPermissionSet xmlns="http://soap.sforce.com/2006/04/metadata">` +
		`<objectPermissions><allowEdit>true</allowEdit><object>A__c</object></objectPermissions>` +
		`<objectPermissions><allowDelete>true</allowDelete><object>B__c</object></objectPermissions>` +
		`<label>L</label></MutingPermissionSet>`)
	if got := mutingObjectProblems(data); len(got) != 1 {
		t.Fatalf("got %q, want one error", got)
	}
}

func TestPermissionMutingRejectionsGroupMessages(t *testing.T) {
	const editOnly = `<MutingPermissionSet xmlns="http://soap.sforce.com/2006/04/metadata"><objectPermissions><allowEdit>true</allowEdit><object>X__c</object></objectPermissions><label>L</label></MutingPermissionSet>`
	const modifyOnly = `<MutingPermissionSet xmlns="http://soap.sforce.com/2006/04/metadata"><objectPermissions><modifyAllRecords>true</modifyAllRecords><object>X__c</object></objectPermissions><label>L</label></MutingPermissionSet>`
	group := func(mutes ...string) string {
		out := `<PermissionSetGroup xmlns="http://soap.sforce.com/2006/04/metadata"><label>G</label>`
		for _, m := range mutes {
			out += `<mutingPermissionSets>` + m + `</mutingPermissionSets>`
		}
		return out + `<permissionSets>Grant</permissionSets></PermissionSetGroup>`
	}
	cases := map[string]struct {
		files map[string]string
		want  []string
	}{
		"two invalid sets": {map[string]string{"a/MuteA.mutingpermissionset": editOnly, "a/MuteB.mutingpermissionset": editOnly, "g/G.permissionsetgroup": group("MuteA", "MuteB")},
			[]string{"To mute Edit, first mute Delete and Modify All Records.", "To mute Edit, first mute Delete and Modify All Records.", mutingPermissionSetGroupLimitMessage}},
		"two valid sets": {map[string]string{"a/MuteA.mutingpermissionset": modifyOnly, "a/MuteB.mutingpermissionset": modifyOnly, "g/G.permissionsetgroup": group("MuteA", "MuteB")},
			[]string{mutingPermissionSetGroupLimitMessage}},
		"one invalid set": {map[string]string{"a/MuteA.mutingpermissionset": editOnly, "g/G.permissionsetgroup": group("MuteA")},
			[]string{"To mute Edit, first mute Delete and Modify All Records.", "Cannot create Permission Set Group Components since the following permission set names are invalid: MuteA"}},
		"one valid set": {map[string]string{"a/MuteA.mutingpermissionset": modifyOnly, "g/G.permissionsetgroup": group("MuteA")}, nil},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var p Project
			for _, path := range []string{"a/MuteA.mutingpermissionset", "a/MuteB.mutingpermissionset"} {
				if _, ok := tc.files[path]; ok {
					p.MutingPermissionSetFiles = append(p.MutingPermissionSetFiles, path)
				}
			}
			p.PermissionSetGroupFiles = []string{"g/G.permissionsetgroup"}
			var got []string
			for _, r := range PermissionMutingRejections(p, func(path string) ([]byte, error) { return []byte(tc.files[path]), nil }) {
				got = append(got, r.Message)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}
