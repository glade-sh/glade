package sema

import (
	"strings"
	"sync"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

var standardObjectShapeOrgs sync.Map // feature signature -> *storage.OrgState

// semaStandardObjectAbsentFromShape reports that the project declares an org
// shape and that shape lacks the feature a standard object needs. Native rows
// (standard_object_shape_compile): a plain Developer org rejects every static
// reference to such an object at compile time, anonymous and named, API 62.0
// and 67.0: "Invalid type: X" for type positions, `new`, DML and class
// literals, "Invalid type: Schema.X" for a static SOQL FROM, and "Variable does
// not exist: X" for X.SObjectType. Dynamic SOQL, describe calls and
// Type.forName stay run-time answers. An unspecified or empty shape cannot
// establish absence, so it stays ungated. The runtime
// predicate is storage.OrgState.AllowsStandardObject.
func semaStandardObjectAbsentFromShape(features []string, name string) bool {
	if len(features) == 0 {
		return false
	}
	for _, feature := range features {
		if feature == project.UnspecifiedStandardFieldShape {
			return false
		}
	}
	name = strings.TrimSpace(name)
	if len(name) > len("schema.") && strings.EqualFold(name[:len("schema.")], "schema.") {
		name = name[len("schema."):]
	}
	if name == "" || strings.ContainsAny(name, ".<>[](), ") {
		return false
	}
	canonical, ok := storage.ResolveKnownStandardObjectName(name)
	if !ok {
		return false
	}
	runtime, staticOnly := splitStaticOnlyShapeFeatures(features)
	if len(runtime) == 0 {
		return false
	}
	if semaShapeOrg(runtime).AllowsStandardObject(canonical) {
		return false
	}
	// A feature only an unreferenced definition enables cannot prove absence.
	if len(staticOnly) > 0 && semaShapeOrg(append(append([]string(nil), runtime...), staticOnly...)).AllowsStandardObject(canonical) {
		return false
	}
	return true
}

func splitStaticOnlyShapeFeatures(features []string) (runtime, staticOnly []string) {
	for _, feature := range features {
		if strings.HasPrefix(feature, project.StaticOnlyShapeFeaturePrefix) {
			staticOnly = append(staticOnly, strings.TrimPrefix(feature, project.StaticOnlyShapeFeaturePrefix))
		} else {
			runtime = append(runtime, feature)
		}
	}
	return runtime, staticOnly
}

func semaShapeOrg(features []string) *storage.OrgState {
	key := strings.Join(features, ",")
	if cached, ok := standardObjectShapeOrgs.Load(key); ok {
		return cached.(*storage.OrgState)
	}
	org := storage.NewOrgState()
	storage.ApplyProjectOrgShape(&org, features)
	actual, _ := standardObjectShapeOrgs.LoadOrStore(key, &org)
	return actual.(*storage.OrgState)
}

// standardObjectAbsentFromDeclaredShape applies the gate to a type reference
// unless a project class or a platform class (Network) owns the spelling.
func (a *Analyzer) standardObjectAbsentFromDeclaredShape(model *semaTypeMemberView, owner, reference string) bool {
	if model == nil || model.state == nil || model.state.base == nil {
		return false
	}
	features := model.state.base.orgShapeFeatures
	if len(features) == 0 || !semaStandardObjectAbsentFromShape(features, reference) {
		return false
	}
	short := strings.TrimSpace(reference)
	if len(short) > len("schema.") && strings.EqualFold(short[:len("schema.")], "schema.") {
		short = short[len("schema."):]
	}
	// Network is also a System class: native compiles the type spelling in a
	// shape without Networks (standard_object_shape_compile fam_Network_t,
	// fam_Network_n); only its static SOQL is rejected.
	if strings.EqualFold(short, "Network") {
		return false
	}
	// A nested type of the owner, of an enclosing type or of a superclass
	// shadows a standard object of the same name, as does a top-level project
	// or platform type (native: standard_object_shape_compile rows, H07,
	// H09, H13-H16).
	if _, nested := a.scopedNestedType(model, owner, short); nested {
		return false
	}
	if members, known := model.lookupName(short); known && !members.sobject {
		return false
	}
	return true
}
