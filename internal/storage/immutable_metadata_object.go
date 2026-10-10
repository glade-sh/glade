package storage

import "strings"

// immutableMetadataObjectNames is the lower-case set behind
// IsImmutableMetadataObject.
var immutableMetadataObjectNames = map[string]struct{}{
	"apexclass": {}, "apextrigger": {}, "apexpage": {}, "apexcomponent": {},
	"fieldpermissions": {}, "objectpermissions": {}, "setupentityaccess": {},
	"permissionset": {}, "permissionsetgroup": {}, "permissionsetgroupcomponent": {},
	"profile": {}, "userrole": {},
	"recordtype": {}, "layout": {}, "staticresource": {},
	"customapplication": {}, "apptabmember": {}, "tabdefinition": {},
	"entitydefinition": {}, "fielddefinition": {},
}

// immutableMetadataObjectMaxLen bounds the ASCII fast path. ASCII lower-casing
// keeps the length, so a longer ASCII name cannot match.
const immutableMetadataObjectMaxLen = len("permissionsetgroupcomponent")

// isImmutableMetadataObjectName matches strings.ToLower(strings.TrimSpace(name))
// against the set. ASCII names are lower-cased into a stack buffer, so the
// per-object check in CloneRuntimeFrozenShared does not allocate. Other names
// take strings.ToLower, which may change their length.
func isImmutableMetadataObjectName(objectName string) bool {
	name := strings.TrimSpace(objectName)
	if name == "" {
		return false
	}
	var buf [immutableMetadataObjectMaxLen]byte
	for i := 0; i < len(name); i++ {
		c := name[i]
		if c >= 0x80 {
			_, ok := immutableMetadataObjectNames[strings.ToLower(name)]
			return ok
		}
		if i >= len(buf) {
			// Keep scanning: a later non-ASCII byte still needs ToLower.
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
	_, ok := immutableMetadataObjectNames[string(buf[:len(name)])]
	return ok
}
