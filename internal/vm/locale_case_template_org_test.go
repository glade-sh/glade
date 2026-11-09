package vm

import "testing"

// The template fast path (no prior org) must drop a cached String == locale
// decision just as SetOrg does.
func TestSetRuntimeTemplateOrgResetsLocaleCaseCache(t *testing.T) {
	template := runtimeGenerationIndexTemplate(1)
	machine := New(nil)
	machine.PrimeMetadataSchema(&template.Org)
	machine.localeCase = localeCaseCache{valid: true, turkish: true}
	org := template.CloneRuntimeOrg()
	machine.SetRuntimeTemplateOrg(&org)
	if machine.localeCase.valid {
		t.Fatal("SetRuntimeTemplateOrg kept a stale localeCase cache")
	}
}
