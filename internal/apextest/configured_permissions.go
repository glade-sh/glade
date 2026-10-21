package apextest

import (
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

func applyConfiguredTestPermissions(org *storage.OrgState, p project.Project) error {
	var statements strings.Builder
	for _, selection := range []struct {
		names                          []string
		object, field, assignment, key string
	}{
		{p.Test.PermissionSets, "PermissionSet", "Name", "PermissionSetId", "permissionSets"},
		{p.Test.PermissionSetGroups, "PermissionSetGroup", "DeveloperName", "PermissionSetGroupId", "permissionSetGroups"},
	} {
		seen := make(map[storage.ID]bool)
		for _, name := range selection.names {
			id, ok := recordFieldID(org.Objects[selection.object], selection.field, name)
			if !ok {
				return fmt.Errorf("glade.yml: test.%s: %q is not in project metadata", selection.key, name)
			}
			if seen[id] {
				continue
			}
			seen[id] = true
			fmt.Fprintf(&statements, "if([SELECT COUNT() FROM PermissionSetAssignment WHERE AssigneeId=:UserInfo.getUserId() AND %s='%s']==0){insert new PermissionSetAssignment(AssigneeId=UserInfo.getUserId(),%s='%s');}\n", selection.assignment, id, selection.assignment, id)
		}
	}
	if statements.Len() == 0 {
		return nil
	}
	// Execute admin setup on the run's private org clone, before static initializers
	// and @TestSetup. The separate VM leaves test governor counters untouched.
	machine := vm.New(nil)
	machine.SetTraceEnabled(false)
	machine.SetOrg(org)
	machine.EnableTestContext()
	program, err := vm.CompileAnonymous(statements.String())
	if err != nil {
		return fmt.Errorf("glade.yml: test permission assignments: %w", err)
	}
	if _, err := machine.Execute(program); err != nil {
		return fmt.Errorf("glade.yml: test permission assignments: %w", err)
	}
	return nil
}

func applyProjectPermissionSetGroupComponents(org *storage.OrgState, p project.Project, readFile func(string) ([]byte, error)) error {
	selected := make(map[string]bool, len(p.Test.PermissionSetGroups))
	for _, name := range p.Test.PermissionSetGroups {
		selected[name] = true
	}
	engine := dml.NewEngine(org)
	for _, path := range p.PermissionSetGroupFiles {
		name := metadataNameFromPath(path, ".permissionsetgroup-meta.xml", ".permissionsetgroup")
		data, err := readFile(path)
		if err != nil {
			return err
		}
		members, err := project.PermissionSetGroupMembers(data)
		if err != nil {
			if selected[name] {
				return fmt.Errorf("glade.yml: test.permissionSetGroups: %q: %w", name, err)
			}
			continue
		}
		groupID, ok := recordFieldID(org.Objects["PermissionSetGroup"], "DeveloperName", name)
		if !ok {
			continue
		}
		for _, member := range members {
			setID, ok := recordFieldID(org.Objects["PermissionSet"], "Name", member)
			if !ok {
				if selected[name] {
					return fmt.Errorf("glade.yml: test.permissionSetGroups: group %q references missing permission set %q", name, member)
				}
				continue
			}
			exists := false
			for _, record := range org.Objects["PermissionSetGroupComponent"].Records {
				if storageIDValueEqualsText(record.Fields["PermissionSetGroupId"], string(groupID)) && storageIDValueEqualsText(record.Fields["PermissionSetId"], string(setID)) {
					exists = true
					break
				}
			}
			if exists {
				continue
			}
			result := engine.Insert([]storage.Record{{Object: "PermissionSetGroupComponent", Fields: map[string]storage.Value{"PermissionSetGroupId": storage.IDValue(groupID), "PermissionSetId": storage.IDValue(setID)}}})
			if !result[0].Success {
				return fmt.Errorf("permission set group %q: %s", name, result[0].Error)
			}
		}
		if err := engine.RecalculatePermissionSetGroups([]storage.ID{groupID}, nil); err != nil {
			return err
		}
	}
	return nil
}
