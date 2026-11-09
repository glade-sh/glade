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
	dependencySets := dependencyPermissionSetNames(p.ManagedPackageDependencies, map[*Project]bool{})
	for _, name := range p.Test.PermissionSets {
		if _, ok := sets[name]; !ok && !dependencySets[strings.ToLower(name)] {
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

// dependencyPermissionSetNames lists the permission sets of loaded managed
// dependencies, as the unprefixed Name and as ns__Name. A subscriber's org
// holds an installed package's sets, so test.permissionSets may assign them.
func dependencyPermissionSetNames(deps []ManagedPackageDependency, seen map[*Project]bool) map[string]bool {
	out := make(map[string]bool)
	for _, dep := range deps {
		if dep.Project == nil || dep.Status != "loaded" || seen[dep.Project] {
			continue
		}
		seen[dep.Project] = true
		namespace := strings.TrimSpace(dep.Namespace)
		if namespace == "" {
			namespace = strings.TrimSpace(dep.Project.Namespace)
		}
		for name := range permissionMetadataPaths(dep.Project.PermissionSetFiles, ".permissionset-meta.xml", ".permissionset") {
			out[strings.ToLower(name)] = true
			if namespace != "" {
				out[strings.ToLower(namespace+"__"+name)] = true
			}
		}
		for name := range dependencyPermissionSetNames(dep.Project.ManagedPackageDependencies, seen) {
			out[name] = true
		}
	}
	return out
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
	members, muting, err := permissionSetGroupParts(data)
	if err != nil {
		return nil, err
	}
	if len(muting) > 0 {
		return nil, fmt.Errorf("muting permission sets are not supported for configured test assignments")
	}
	return members, nil
}

// PermissionSetGroupGrantingMembers lists the granting sets of a group,
// muted or not.
func PermissionSetGroupGrantingMembers(data []byte) ([]string, error) {
	members, _, err := permissionSetGroupParts(data)
	return members, err
}

func permissionSetGroupParts(data []byte) ([]string, []string, error) {
	var group struct {
		XMLName xml.Name `xml:"PermissionSetGroup"`
		Members []string `xml:"permissionSets"`
		Muting  []string `xml:"mutingPermissionSets"`
	}
	if err := xml.Unmarshal(data, &group); err != nil {
		return nil, nil, err
	}
	for i := range group.Members {
		group.Members[i] = strings.TrimSpace(group.Members[i])
	}
	return group.Members, group.Muting, nil
}

// PermissionSetGroupMutes lists the muting permission sets of a group.
func PermissionSetGroupMutes(data []byte) ([]string, error) {
	var group struct {
		XMLName xml.Name `xml:"PermissionSetGroup"`
		Muting  []string `xml:"mutingPermissionSets"`
	}
	if err := xml.Unmarshal(data, &group); err != nil {
		return nil, err
	}
	for i := range group.Muting {
		group.Muting[i] = strings.TrimSpace(group.Muting[i])
	}
	return group.Muting, nil
}
