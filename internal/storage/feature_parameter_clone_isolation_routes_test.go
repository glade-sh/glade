package storage

import (
	"reflect"
	"testing"
)

func TestFeatureParameterRuntimeTemplateWithoutParameters(t *testing.T) {
	template := NewRuntimeTemplate(NewOrgState())
	first := template.CloneRuntimeOrg()
	first.Metadata.FeatureParameters = append(first.Metadata.FeatureParameters, FeatureParameterMetadata{Name: "Enabled", Type: "Boolean", Value: "true"})
	if template.Org.Metadata.FeatureParameters != nil || template.CloneRuntimeOrg().Metadata.FeatureParameters != nil {
		t.Fatal("adding clone metadata changed an empty template or later clone")
	}
}

func TestFeatureParameterCloneRoutesIsolation(t *testing.T) {
	for _, route := range []string{"Clone", "CloneRuntime", "CloneRuntimeFrozenDefinition", "CloneRuntimeFrozenShared", "RuntimeTemplate.CloneRuntimeOrg"} {
		t.Run(route, func(t *testing.T) {
			source := NewOrgState()
			source.Metadata.FeatureParameters = []FeatureParameterMetadata{
				{Name: "Enabled", Type: "Boolean", DataflowDirection: "SubscriberToLmo", Value: "true"},
				{Name: "Quota", Type: "Integer", DataflowDirection: "SubscriberToLmo", Value: "11"},
				{Name: "StartDate", Type: "Date", DataflowDirection: "SubscriberToLmo", Value: "2020-02-02"},
			}
			want := append([]FeatureParameterMetadata(nil), source.Metadata.FeatureParameters...)
			template := NewRuntimeTemplate(source)
			clone := func() OrgState {
				switch route {
				case "Clone":
					return source.Clone()
				case "CloneRuntime":
					return source.CloneRuntime()
				case "CloneRuntimeFrozenDefinition":
					return source.CloneRuntimeFrozenDefinition()
				case "CloneRuntimeFrozenShared":
					return source.CloneRuntimeFrozenShared()
				default:
					return template.CloneRuntimeOrg()
				}
			}
			first, sibling := clone(), clone()
			for i, value := range []string{"false", "17", "2020-03-04"} {
				first.Metadata.FeatureParameters[i].Value = value
			}
			later := clone()
			for name, org := range map[string]OrgState{"source": source, "template": template.Org, "sibling": sibling, "later": later} {
				if !reflect.DeepEqual(org.Metadata.FeatureParameters, want) {
					t.Errorf("clone write escaped isolation: %s=%v, want %v", name, org.Metadata.FeatureParameters, want)
				}
			}
		})
	}
}
