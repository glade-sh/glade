package apextest

import (
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// OrgFromIndexWithConfiguredPermissions builds the project org and replays the
// permission assignments named in the project's test configuration.
func OrgFromIndexWithConfiguredPermissions(index typesys.Index) (storage.OrgState, error) {
	org := orgFromIndex(index)
	p, err := project.Load(index.Project.Root)
	if err != nil {
		return org, err
	}
	return org, applyConfiguredTestPermissions(&org, p)
}

// ApplyConfiguredTestPermissions assigns the project's configured permission
// sets and groups to the default administrator of an org built outside Run.
func ApplyConfiguredTestPermissions(org *storage.OrgState, p project.Project) error {
	return applyConfiguredTestPermissions(org, p)
}

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
			if !ok && selection.object == "PermissionSet" {
				id, ok = namespacedPermissionSetID(org.Objects[selection.object], name)
			}
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
		var mutes []string
		if err != nil {
			if selected[name] {
				return fmt.Errorf("glade.yml: test.permissionSetGroups: %q: %w", name, err)
			}
			// A group with muting sets is seeded for assignments the test
			// code makes itself; only the configured route rejects it.
			granting, grantErr := project.PermissionSetGroupGrantingMembers(data)
			muting, muteErr := project.PermissionSetGroupMutes(data)
			if grantErr != nil || muteErr != nil || len(muting) == 0 {
				continue
			}
			members, mutes = granting, muting
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
		for _, mute := range mutes {
			muteID, ok := seedProjectMutingPermissionSet(org, p, mute)
			if !ok {
				continue
			}
			exists := false
			for _, record := range org.Objects["PermissionSetGroupComponent"].Records {
				if storageIDValueEqualsText(record.Fields["PermissionSetGroupId"], string(groupID)) && storageIDValueEqualsText(record.Fields["PermissionSetId"], string(muteID)) {
					exists = true
					break
				}
			}
			if exists {
				continue
			}
			result := engine.Insert([]storage.Record{{Object: "PermissionSetGroupComponent", Fields: map[string]storage.Value{"PermissionSetGroupId": storage.IDValue(groupID), "PermissionSetId": storage.IDValue(muteID)}}})
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

// namespacedPermissionSetID resolves ns__Name to an installed package's
// permission set, whose Name is stored unprefixed with NamespacePrefix set.
func namespacedPermissionSetID(state storage.ObjectState, name string) (storage.ID, bool) {
	idx := strings.Index(name, "__")
	if idx <= 0 || idx+2 >= len(name) {
		return "", false
	}
	namespace, local := name[:idx], name[idx+2:]
	for _, record := range state.Records {
		if strings.EqualFold(record.Fields["Name"].String, local) && strings.EqualFold(record.Fields["NamespacePrefix"].String, namespace) {
			return record.ID, true
		}
	}
	return "", false
}

// seedProjectMutingPermissionSet materializes a deployed muting permission set
// and its object and field rows, parented on the MutingPermissionSet record.
func seedProjectMutingPermissionSet(org *storage.OrgState, p project.Project, name string) (storage.ID, bool) {
	var file string
	for _, candidate := range p.MutingPermissionSetFiles {
		if metadataNameFromPath(candidate, ".mutingpermissionset-meta.xml", ".mutingpermissionset") == name {
			file = candidate
			break
		}
	}
	if file == "" {
		return "", false
	}
	storage.EnsureStandardObject(org, "MutingPermissionSet")
	state := org.Objects["MutingPermissionSet"]
	if state.Records == nil {
		state.Records = make(map[storage.ID]storage.Record)
	}
	if id, ok := recordFieldID(state, "DeveloperName", name); ok {
		return id, true
	}
	if org.IDSequences == nil {
		org.IDSequences = make(map[string]uint64)
	}
	generator := storage.NewStandardIDGenerator()
	generator.Sequences = org.IDSequences
	id, err := generator.Next("MutingPermissionSet")
	if err != nil {
		return "", false
	}
	cache := make(map[string]permissionSetMetadataCacheEntry)
	label := name
	if metadata, ok := readPermissionSetMetadata(file, cache); ok {
		if strings.TrimSpace(metadata.Label) != "" {
			label = strings.TrimSpace(metadata.Label)
		}
		// A deployed Read-only field mute is discarded by Salesforce. Keep
		// Edit-only and Read+Edit entries, including in the same muting set.
		// permset_muting_read_only and permset_muting_multiple capture this
		// independently at source API 62 and 67.
		filtered := make([]permissionSetFieldPermission, 0, len(metadata.FieldPermissions))
		for _, field := range metadata.FieldPermissions {
			if field.Readable && !field.Editable {
				continue
			}
			filtered = append(filtered, field)
		}
		metadata.FieldPermissions = filtered
		cache[file] = permissionSetMetadataCacheEntry{metadata: metadata, ok: true}
	}
	state.Records[id] = storage.Record{ID: id, Object: "MutingPermissionSet", Fields: map[string]storage.Value{
		"DeveloperName": storage.StringValue(name),
		"MasterLabel":   storage.StringValue(label),
	}}
	org.Objects["MutingPermissionSet"] = state
	applyProjectPermissionSetMetadataPermissions(org, file, string(id), &generator, objectPermissionRecordKeys(org.Objects["ObjectPermissions"]), fieldPermissionRecordKeys(org.Objects["FieldPermissions"]), cache)
	org.IDSequences = generator.Sequences
	return id, true
}
