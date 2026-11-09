package apextest

import (
	"path/filepath"
	"testing"
)

// writeSyntheticProbeAccess grants the running administrator the named custom
// object and field access that a deployed org would need before plain DML at
// source API 62 through 67 can write optional custom fields.
func writeSyntheticProbeAccess(t *testing.T, root, object string, fields ...string) {
	t.Helper()
	xml := `<PermissionSet xmlns="http://soap.sforce.com/2006/04/metadata"><label>Synthetic Probe Access</label>`
	for _, field := range fields {
		xml += `<fieldPermissions><editable>true</editable><field>` + object + `.` + field + `</field><readable>true</readable></fieldPermissions>`
	}
	xml += `<objectPermissions><allowCreate>true</allowCreate><allowDelete>true</allowDelete><allowEdit>true</allowEdit><allowRead>true</allowRead><object>` + object + `</object></objectPermissions></PermissionSet>`
	writeFile(t, filepath.Join(root, "force-app/main/default/permissionsets/SyntheticProbeAccess.permissionset-meta.xml"), xml)
	writeFile(t, filepath.Join(root, "glade.yml"), "test:\n  permissionSets: [SyntheticProbeAccess]\n")
}
