package apextest

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

func TestInitializedTestOrgRetainsSchemaGeneration(t *testing.T) {
	org := storage.NewOrgState()
	for _, name := range []string{"PermissionSetAssignment", "Group", "Task"} {
		storage.EnsureStandardObject(&org, name)
	}
	initializeTestOrg(&org)
	if org.RuntimeSchemaStamp == "" {
		t.Fatal("test initialization discarded the trusted runtime schema generation")
	}
	base := vm.New(nil)
	base.PrimeMetadataSchema(&org)
	generation := org.RuntimeGenerationID()
	for range 3 {
		clone := org.CloneRuntimeFrozenShared()
		machine := base.CloneRuntime(nil)
		machine.SetRuntimeTemplateOrg(&clone)
		for _, name := range []string{"PermissionSetAssignment", "Group", "Task"} {
			storage.EnsureStandardObject(&clone, name)
		}
		if clone.RuntimeSchemaStamp != org.RuntimeSchemaStamp || clone.RuntimeGenerationID() != generation {
			t.Fatal("ordinary worker initialization or standard-object ensure discarded shared schema caches")
		}
	}
}

func TestInitializedTestOrgPreparesExistingStandardDefinitions(t *testing.T) {
	seed := storage.NewOrgState()
	for _, name := range []string{"Account", "Contact", "PermissionSetAssignment"} {
		seed.Objects[name] = storage.ObjectState{Definition: storage.ObjectDefinition{
			APIName: name,
			Fields:  map[string]storage.Field{"Owned__c": {APIName: "Owned__c", Type: storage.FieldString}},
		}}
	}
	org := seed.CloneRuntimeFrozenShared()
	initializeTestOrg(&org)
	generation := org.RuntimeGenerationID()
	for _, name := range []string{"Account", "Contact", "PermissionSetAssignment"} {
		if storage.StandardObjectFieldsNeedWrite(org.Objects[name].Definition) {
			t.Fatalf("%s still needs lazy field installation", name)
		}
		if _, ok := org.Objects[name].Definition.Fields["Owned__c"]; !ok {
			t.Fatal("preparation lost project fields")
		}
		if len(seed.Objects[name].Definition.Fields) != 1 {
			t.Fatal("preparation wrote into the source template")
		}
		storage.EnsureStandardObject(&org, name)
		if org.RuntimeGenerationID() != generation || org.RuntimeSchemaStamp == "" {
			t.Fatalf("%s invalidated the prepared schema", name)
		}
	}
}

// A project with enabled org-shape features prepares every standard definition
// with that feature signature. Lazy ensures from runAs, DML and SOQL on test
// clones must then be no-ops that keep the shared schema generation, exactly
// as they are for a plain org.
func TestInitializedFeatureOrgRetainsSchemaGeneration(t *testing.T) {
	org := storage.NewOrgState()
	storage.ApplyOrgShape(&org, []string{"Chatter", "EnableSetPasswordInApi", "LightningExperience"})
	names := []string{"PermissionSetAssignment", "EmailMessage", "EntityDefinition", "Group", "Account", "User"}
	for _, name := range names {
		storage.EnsureStandardObject(&org, name)
	}
	initializeTestOrg(&org)
	if org.RuntimeSchemaStamp == "" {
		t.Fatal("test initialization discarded the trusted runtime schema generation")
	}
	base := vm.New(nil)
	base.PrimeMetadataSchema(&org)
	generation := org.RuntimeGenerationID()
	for range 3 {
		clone := org.CloneRuntimeFrozenShared()
		machine := base.CloneRuntime(nil)
		machine.SetRuntimeTemplateOrg(&clone)
		for _, name := range names {
			storage.EnsureStandardObject(&clone, name)
			if clone.RuntimeSchemaStamp != org.RuntimeSchemaStamp || clone.RuntimeGenerationID() != generation {
				t.Fatalf("%s: standard-object ensure on a feature-enabled test clone discarded shared schema caches", name)
			}
		}
	}
}

// Every test enables the test context, which merges the async platform objects
// into its org. On a project with a scratch org definition the standard field
// policy removes AsyncApexJob audit fields that those helpers would add, so the
// merge must follow the policy: otherwise each test re-adds them and drops the
// shared schema generation.
func TestInitializedShapedProjectTestContextRetainsSchemaGeneration(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
	writeFile(t, filepath.Join(root, "config", "project-scratch-def.json"), `{"orgName":"Shaped","edition":"Developer"}`)
	writeFile(t, filepath.Join(root, "force-app", "main", "default", "classes", "ShapedProbe.cls"), "public class ShapedProbe {}")
	writeFile(t, filepath.Join(root, "force-app", "main", "default", "classes", "ShapedProbe.cls-meta.xml"), "<ApexClass><apiVersion>67.0</apiVersion></ApexClass>")
	org := orgFromIndex(loadTestIndex(t, root))
	initializeTestOrg(&org)
	if org.RuntimeSchemaStamp == "" {
		t.Fatal("test initialization discarded the trusted runtime schema generation")
	}
	base := vm.New(nil)
	base.PrimeMetadataSchema(&org)
	generation := org.RuntimeGenerationID()
	for range 3 {
		clone := org.CloneRuntimeFrozenShared()
		machine := base.CloneRuntime(nil)
		machine.SetRuntimeTemplateOrg(&clone)
		machine.EnableTestContext()
		if clone.RuntimeSchemaStamp != org.RuntimeSchemaStamp || clone.RuntimeGenerationID() != generation {
			t.Fatal("test context on a shaped project's test clone discarded shared schema caches")
		}
		for _, name := range []string{"AsyncApexJob", "CronTrigger", "ApexClass"} {
			storage.EnsureStandardObject(&clone, name)
			if clone.RuntimeSchemaStamp != org.RuntimeSchemaStamp || clone.RuntimeGenerationID() != generation {
				t.Fatalf("%s: standard-object ensure after the test context discarded shared schema caches", name)
			}
		}
	}
}
