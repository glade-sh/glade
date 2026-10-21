package soql

import "strings"

// IsUserGroupContactField identifies the contact-field projection observed on
// references whose only two targets are User and Group. Access-mode and query
// position restrictions are applied by the caller.
func IsUserGroupContactField(targets []string, leaf string) bool {
	if len(targets) != 2 ||
		!(strings.EqualFold(targets[0], "User") && strings.EqualFold(targets[1], "Group") ||
			strings.EqualFold(targets[0], "Group") && strings.EqualFold(targets[1], "User")) {
		return false
	}
	switch strings.ToLower(leaf) {
	case "firstname", "lastname", "phone", "email":
		return true
	default:
		return false
	}
}
