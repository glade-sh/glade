package project

import (
	"encoding/xml"
	"os"
	"strings"
)

// PermissionMutingRejection names one muting permission set or permission set
// group file that Salesforce rejects at deploy time. Message keeps the native
// component error text.
type PermissionMutingRejection struct {
	File    string
	Message string
}

// mutingObjectPermission is one objectPermissions entry of a muting permission
// set. A true flag mutes that permission.
type mutingObjectPermission struct {
	AllowCreate      string `xml:"allowCreate"`
	AllowDelete      string `xml:"allowDelete"`
	AllowEdit        string `xml:"allowEdit"`
	AllowRead        string `xml:"allowRead"`
	ModifyAllRecords string `xml:"modifyAllRecords"`
	ViewAllRecords   string `xml:"viewAllRecords"`
}

type mutingDependency struct {
	label string
	muted func(mutingObjectPermission) bool
}

func mutingFlag(value string) bool { return strings.EqualFold(strings.TrimSpace(value), "true") }

var (
	mutingCreate    = mutingDependency{"Create", func(o mutingObjectPermission) bool { return mutingFlag(o.AllowCreate) }}
	mutingEdit      = mutingDependency{"Edit", func(o mutingObjectPermission) bool { return mutingFlag(o.AllowEdit) }}
	mutingDelete    = mutingDependency{"Delete", func(o mutingObjectPermission) bool { return mutingFlag(o.AllowDelete) }}
	mutingViewAll   = mutingDependency{"View All Records", func(o mutingObjectPermission) bool { return mutingFlag(o.ViewAllRecords) }}
	mutingModifyAll = mutingDependency{"Modify All Records", func(o mutingObjectPermission) bool { return mutingFlag(o.ModifyAllRecords) }}
)

// mutingObjectRules are the object permission dependencies that native
// deployment enforces: muting a permission requires the permissions that
// depend on it to be muted in the same set. message is the captured component
// error, emitted whenever any dependency is unmuted (dot-30-permset-muting-modes
// and permset_muting_partial: partly muted dependencies give the same
// sentence). Muting View All Records without Modify All Records fails with a
// native missing-label text, kept verbatim.
//
// The order is the captured precedence inside one object: Read before Edit
// (permset_muting_partial C03), Edit before Delete (C01), Delete before View
// All (X12) and Edit before View All (X13).
var mutingObjectRules = []struct {
	muted    func(mutingObjectPermission) bool
	requires []mutingDependency
	message  string
}{
	{func(o mutingObjectPermission) bool { return mutingFlag(o.AllowRead) }, []mutingDependency{mutingCreate, mutingEdit, mutingDelete, mutingViewAll, mutingModifyAll},
		"To mute Read, first mute Create, Edit, Delete, View All Records, and Modify All Records."},
	{mutingEdit.muted, []mutingDependency{mutingDelete, mutingModifyAll},
		"To mute Edit, first mute Delete and Modify All Records."},
	{mutingDelete.muted, []mutingDependency{mutingModifyAll},
		"To mute Delete, first mute Modify All Records."},
	{mutingViewAll.muted, []mutingDependency{mutingModifyAll},
		"__MISSING LABEL__ PropertyFile - val View All Records not found in section Page_ObjectPermissions_Edit_Muting"},
}

// mutingPermissionSetGroupLimitMessage is the group component error when a
// group names more than one muting permission set, valid or not
// (permset_muting_partial C11 and X11b).
const mutingPermissionSetGroupLimitMessage = "There's already a muting permission set for this permission set group. You can't add another muting permission set, but you can edit the existing one."

// mutingObjectProblems lists the native object dependency errors of one muting
// permission set document. Native deployment reports one error per muting
// set. Inside an object the rule order decides it. When several objects
// violate a rule, native picks one of them and the pick changed between
// identical deployments (permset_muting_partial X10b), so Glade reports the
// first violating object in file order.
func mutingObjectProblems(data []byte) []string {
	var set struct {
		XMLName xml.Name                 `xml:"MutingPermissionSet"`
		Objects []mutingObjectPermission `xml:"objectPermissions"`
	}
	if xml.Unmarshal(data, &set) != nil {
		return nil
	}
	for _, object := range set.Objects {
		for _, rule := range mutingObjectRules {
			if !rule.muted(object) {
				continue
			}
			for _, dep := range rule.requires {
				if !dep.muted(object) {
					return []string{rule.message}
				}
			}
		}
	}
	return nil
}

// PermissionMutingMetadataFiles lists the files whose contents decide
// PermissionMutingRejections.
func PermissionMutingMetadataFiles(p Project) []string {
	files := append([]string(nil), p.MutingPermissionSetFiles...)
	return append(files, p.PermissionSetGroupFiles...)
}

// PermissionMutingRejections reports the muting permission sets that native
// deployment rejects and the permission set groups that name them. Salesforce
// reports both components as failed in one deployment.
func PermissionMutingRejections(p Project, read func(string) ([]byte, error)) []PermissionMutingRejection {
	if len(p.MutingPermissionSetFiles) == 0 {
		return nil
	}
	if read == nil {
		read = os.ReadFile
	}
	var rejections []PermissionMutingRejection
	invalid := map[string]bool{}
	for _, file := range p.MutingPermissionSetFiles {
		data, err := read(file)
		if err != nil {
			continue
		}
		problems := mutingObjectProblems(data)
		if len(problems) == 0 {
			continue
		}
		name := metadataFileName(file, ".mutingpermissionset-meta.xml", ".mutingpermissionset")
		invalid[strings.ToLower(name)] = true
		for _, problem := range problems {
			rejections = append(rejections, PermissionMutingRejection{File: file, Message: problem})
		}
	}
	for _, file := range p.PermissionSetGroupFiles {
		data, err := read(file)
		if err != nil {
			continue
		}
		mutes, err := PermissionSetGroupMutes(data)
		if err != nil {
			continue
		}
		if len(mutes) > 1 {
			rejections = append(rejections, PermissionMutingRejection{File: file, Message: mutingPermissionSetGroupLimitMessage})
			continue
		}
		if len(mutes) == 1 && invalid[strings.ToLower(mutes[0])] {
			rejections = append(rejections, PermissionMutingRejection{
				File:    file,
				Message: "Cannot create Permission Set Group Components since the following permission set names are invalid: " + mutes[0],
			})
		}
	}
	return rejections
}

func metadataFileName(path string, suffixes ...string) string {
	name := path
	if at := strings.LastIndexAny(name, `/\`); at >= 0 {
		name = name[at+1:]
	}
	for _, suffix := range suffixes {
		if strings.HasSuffix(name, suffix) {
			return strings.TrimSuffix(name, suffix)
		}
	}
	return name
}
