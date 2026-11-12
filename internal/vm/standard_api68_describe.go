package vm

import (
	"maps"
	"reflect"
	"slices"
	"strings"

	"github.com/glade-sh/glade/internal/apexversion"
	"github.com/glade-sh/glade/internal/storage"
)

// Captured API68 changes apply only to an exact observed root shape. A source
// without a declared edition or with an unobserved combination stays on the
// existing catalog rather than acquiring an inferred profile.
func (vm *VM) standardAPI68Context() (profile string, major int, named, known bool) {
	if vm == nil || vm.Org == nil {
		return "", 0, false, false
	}
	profile, known = vm.standardAPI68Profile()
	if !known {
		return "", 0, false, false
	}
	version := vm.currentMethod.APIVersion
	if version == "" {
		version = vm.Org.APIVersion
	}
	major, ok := apexversion.Major(version)
	if !ok || major < 62 || major > 68 {
		return "", 0, false, false
	}
	return profile, major, vm.currentClass != "" && !vm.currentTrigger, true
}

// standardAPI68ProfileMemo remembers the profile selected for one feature list.
// Org.Features is replaced, never edited in place, so an element-wise match
// proves the memo still describes the org.
type standardAPI68ProfileMemo struct {
	features []string
	profile  string
	known    bool
}

// standardAPI68Profile resolves the captured profile for the org's features.
// Every describe cache key asks for it, so the canonical signature is built
// once per feature list instead of once per describe.
func (vm *VM) standardAPI68Profile() (string, bool) {
	features := vm.Org.Features
	if memo := vm.standardAPI68Memo; memo != nil && slices.Equal(memo.features, features) {
		return memo.profile, memo.known
	}
	profile, known := storage.StandardAPI68DescribeProfile(features)
	vm.standardAPI68Memo = &standardAPI68ProfileMemo{features: slices.Clone(features), profile: profile, known: known}
	return profile, known
}

// Apply the observed global additions to a private per-call view. The cached
// base remains usable by callers at other source API versions.
func (vm *VM) projectStandardAPI68Global(value Value) Value {
	profile, major, named, ok := vm.standardAPI68Context()
	if !ok {
		return value
	}
	names := storage.StandardAPI68DescribeObjectNames(profile, named)
	if len(names) == 0 {
		return value
	}
	changed := false
	projected := value
	for _, name := range names {
		for _, alias := range vm.schemaDescribeMapAliases(name) {
			key := mapKey(String(alias))
			_, exists := projected.Map[key]
			if (major == 68 && exists) || (major < 68 && !exists) {
				continue
			}
			if !changed {
				projected.Map = maps.Clone(value.Map)
				projected.mapEntriesShared = false
				projected.Ref = newValueRef()
				changed = true
			}
			if major == 68 {
				vm.putSchemaGlobalDescribeAlias(&projected, alias, name, sObjectTypeToken(name))
			} else {
				delete(projected.Map, key)
			}
		}
	}
	if changed {
		fillDescribeMapStoredKeys(&projected)
	}
	return projected
}

// Apply field observations only after the shared prepared baseline is loaded,
// so another API version or source route cannot reuse this projection.
func (vm *VM) projectStandardAPI68Fields(name string, definition, source storage.ObjectDefinition) storage.ObjectDefinition {
	profile, major, named, ok := vm.standardAPI68Context()
	if !ok || major != 68 {
		return definition
	}
	fields := storage.StandardAPI68DescribeFields(profile, name, named)
	if object, exists := storage.StandardAPI68DescribeObjectForRoute(profile, name, named); exists {
		// A newly advertised object's observed field map supersedes the older
		// generated baseline. Retain explicit project deltas, which are not
		// established absent by the clean-scratch capture.
		definition = cloneDescribeObjectDefinition(definition)
		definition.Fields = object.Fields
		if vm.Org != nil {
			if _, present := vm.resolveObjectName(name); present {
				baseline, hasBaseline := storage.StandardObjectDefinition(name)
				for fieldName, field := range source.Fields {
					if _, already := storage.ResolveFieldName(definition, "", fieldName); already {
						continue
					}
					oldName, oldExists := storage.ResolveFieldName(baseline, "", fieldName)
					configured, recorded := storage.StandardAPI68ConfiguredField(source, fieldName)
					if !recorded || configured || !hasBaseline || !oldExists || !reflect.DeepEqual(field, baseline.Fields[oldName]) {
						definition.Fields[fieldName] = field
					}
				}
			}
		}
		return definition
	}
	if len(fields) == 0 {
		return definition
	}
	definition = cloneDescribeObjectDefinition(definition)
	if definition.Fields == nil {
		definition.Fields = map[string]storage.Field{}
	}
	for name, field := range fields {
		if _, exists := storage.ResolveFieldName(definition, "", name); !exists {
			definition.Fields[name] = field
		}
	}
	return definition
}

func (vm *VM) isStandardAPI68NewObject(name string) bool {
	_, ok := vm.standardAPI68ObjectDefinition(name)
	return ok
}

func (vm *VM) standardAPI68ObjectDefinition(name string) (storage.ObjectDefinition, bool) {
	profile, major, named, ok := vm.standardAPI68Context()
	if !ok || major != 68 {
		return storage.ObjectDefinition{}, false
	}
	return storage.StandardAPI68DescribeObjectForRoute(profile, strings.TrimSpace(name), named)
}

func (vm *VM) standardAPI68ObjectHidden(name string) bool {
	profile, major, named, ok := vm.standardAPI68Context()
	if !ok || major >= 68 {
		return false
	}
	_, observed := storage.StandardAPI68DescribeObjectForRoute(profile, strings.TrimSpace(name), named)
	return observed
}

// The captured named-only fields do not inherit the administrator shortcut:
// Salesforce exposed their read access only after an explicit field grant.
func (vm *VM) standardAPI68ExplicitFieldRead(object, field string) bool {
	if vm == nil || vm.Org == nil {
		return false
	}
	user := vm.executionUser
	if vm.testContext != nil && vm.testContext.CurrentUser.Kind != "" {
		user = vm.testContext.CurrentUser
	}
	profileID := stringField(user, "ProfileId")
	parents := vm.assignedPermissionSetIDs(stringField(user, "Id"))
	parents = append(parents, vm.profileOwnedPermissionSetID(profileID), profileID)
	for _, parent := range parents {
		if allowed, observed := vm.explicitFieldPermission(parent, object, field, "isAccessible"); observed && allowed {
			return true
		}
	}
	return false
}
