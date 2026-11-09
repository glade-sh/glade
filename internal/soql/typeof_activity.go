package soql

import (
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// orgWithTypeofActivityTargets supplies project-owned activity-enabled custom
// targets to TYPEOF without mutating the shared organization or cached schema.
func orgWithTypeofActivityTargets(org storage.OrgState, objectName string, query Query) storage.OrgState {
	if !strings.EqualFold(objectName, "Task") && !strings.EqualFold(objectName, "Event") {
		return org
	}
	targets := map[string]bool{}
	for _, spec := range query.Typeofs {
		if !strings.EqualFold(spec.Relationship, "What") {
			continue
		}
		for name := range spec.When {
			canonical, ok := storage.ResolveObjectName(org, name)
			if !ok {
				continue
			}
			def := org.Objects[canonical].Definition
			if strings.HasSuffix(strings.ToLower(canonical), "__c") && strings.EqualFold(def.Metadata["enableActivities"], "true") {
				targets[canonical] = true
			}
		}
	}
	if len(targets) == 0 {
		return org
	}
	names := make([]string, 0, len(targets))
	for name := range targets {
		names = append(names, name)
	}
	sort.Strings(names)
	objects := make(map[string]storage.ObjectState, len(org.Objects))
	for name, state := range org.Objects {
		objects[name] = state
	}
	state := objects[objectName]
	state.Definition = state.Definition.Clone()
	for i := range state.Definition.Relations {
		rel := &state.Definition.Relations[i]
		if !strings.EqualFold(rel.Field, "WhatId") || !strings.EqualFold(rel.ParentRelationship, "What") {
			continue
		}
		for _, name := range names {
			found := false
			for _, existing := range rel.ParentObjects {
				if strings.EqualFold(existing, name) {
					found = true
					break
				}
			}
			if !found {
				rel.ParentObjects = append(rel.ParentObjects, name)
			}
		}
	}
	objects[objectName] = state
	org.Objects = objects
	return org
}
