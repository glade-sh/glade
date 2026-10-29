package storage

import "github.com/glade-sh/glade/internal/project"

// Only feature transitions captured in org_object_availability/org.tsv belong
// here. Other catalog objects keep their existing policy until native A/B rows
// establish their feature or setting trigger.
var standardObjectFeatures = []struct{ object, feature string }{
	{"Quote", "Quotes"},
	{"QuoteLineItem", "Quotes"},
	{"WebStore", "OrderManagement"},
}

// AllowsStandardObject checks the declared shape, not whether an object is in
// the catalog or physically hydrated. An unspecified project cannot establish
// feature absence. Captured provisioning alternatives share the constructor
// policy so compilation and runtime availability agree.
func (o *OrgState) AllowsStandardObject(objectName string) bool {
	for _, gate := range standardObjectFeatures {
		if stringsEqualFold(objectName, gate.object) {
			if o == nil {
				return false
			}
			if o.HasFeature(project.UnspecifiedStandardFieldShape) {
				return true
			}
			if policy := standardFieldPolicyFor(objectName); policy != nil &&
				(len(policy.RequiredFeatures) > 0 || len(policy.RequiredFeaturesAnyOf) > 0) {
				return StandardObjectConstructorAvailableForFeatures(objectName, o.Features)
			}
			return o.HasFeature(gate.feature)
		}
	}
	return true
}

func refreshOrgFeatureObjects(org *OrgState) {
	for _, gate := range standardObjectFeatures {
		if !org.AllowsStandardObject(gate.object) {
			DeleteObject(org, gate.object)
		} else if _, exists := org.Objects[gate.object]; !exists {
			EnsureStandardObject(org, gate.object)
		}
	}
}
