package project

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func validateTestPermissions(p Project) error {
	sets := permissionMetadataPaths(p.PermissionSetFiles, ".permissionset-meta.xml", ".permissionset")
	groups := permissionMetadataPaths(p.PermissionSetGroupFiles, ".permissionsetgroup-meta.xml", ".permissionsetgroup")
	for _, name := range p.Test.PermissionSets {
		if _, ok := sets[name]; !ok {
			return fmt.Errorf("glade.yml: test.permissionSets: permission set %q is not in project metadata", name)
		}
	}
	for _, name := range p.Test.PermissionSetGroups {
		path, ok := groups[name]
		if !ok {
			return fmt.Errorf("glade.yml: test.permissionSetGroups: permission set group %q is not in project metadata", name)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		members, err := PermissionSetGroupMembers(data)
		if err != nil {
			return fmt.Errorf("glade.yml: test.permissionSetGroups: %q: %w", name, err)
		}
		for _, member := range members {
			if _, ok := sets[member]; !ok {
				return fmt.Errorf("glade.yml: test.permissionSetGroups: group %q references permission set %q not in project metadata", name, member)
			}
		}
	}
	return nil
}

func permissionMetadataPaths(paths []string, suffixes ...string) map[string]string {
	out := make(map[string]string, len(paths))
	for _, path := range paths {
		name := filepath.Base(path)
		for _, suffix := range suffixes {
			if strings.HasSuffix(name, suffix) {
				out[strings.TrimSuffix(name, suffix)] = path
				break
			}
		}
	}
	return out
}

// PermissionSetGroupMembers reads the ordinary, unmuted group shape supported
// by the runtime. Reject muting rather than silently granting muted permissions.
func PermissionSetGroupMembers(data []byte) ([]string, error) {
	var group struct {
		XMLName xml.Name `xml:"PermissionSetGroup"`
		Members []string `xml:"permissionSets"`
		Muting  []string `xml:"mutingPermissionSets"`
	}
	if err := xml.Unmarshal(data, &group); err != nil {
		return nil, err
	}
	if len(group.Muting) > 0 {
		return nil, fmt.Errorf("muting permission sets are not supported for configured test assignments")
	}
	for i := range group.Members {
		group.Members[i] = strings.TrimSpace(group.Members[i])
	}
	return group.Members, nil
}
