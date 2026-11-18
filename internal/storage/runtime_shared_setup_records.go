package storage

import "strings"

// runtimeSharedSetupObjects are read-only setup objects whose rows are written
// only while the base org is built (seed data, project setup metadata and the
// compiled type-implementor enumeration). No test runtime path edits their
// rows; a DML write goes through EnsureMutableObjectRecords, which copies the
// rows before the write, as it does for the immutable metadata objects.
var runtimeSharedSetupObjects = map[string]struct{}{
	"apextypeimplementor": {}, "userlicense": {}, "permissionsetlicense": {},
	"opportunitystage": {}, "taskstatus": {}, "leadstatus": {}, "taskpriority": {},
	"duplicaterule": {}, "matchingrule": {},
}

// sharesSetupRecordsAtRuntimeClone reports whether CloneRuntimeFrozenShared
// shares this object's rows by reference with the clone. An empty object keeps
// its own map: there is nothing to save.
func sharesSetupRecordsAtRuntimeClone(name string, object ObjectState) bool {
	if len(object.Records) == 0 {
		return false
	}
	return isRuntimeSharedSetupObject(object.Definition.APIName) || isRuntimeSharedSetupObject(name)
}

// isRuntimeSharedSetupObject matches strings.ToLower(name) against the set
// without allocating for ASCII names.
func isRuntimeSharedSetupObject(name string) bool {
	var buf [len("permissionsetlicense")]byte
	for i := 0; i < len(name); i++ {
		c := name[i]
		if c >= 0x80 {
			_, ok := runtimeSharedSetupObjects[strings.ToLower(name)]
			return ok
		}
		if i >= len(buf) {
			continue
		}
		if 'A' <= c && c <= 'Z' {
			c += 'a' - 'A'
		}
		buf[i] = c
	}
	if len(name) > len(buf) {
		return false
	}
	_, ok := runtimeSharedSetupObjects[string(buf[:len(name)])]
	return ok
}
