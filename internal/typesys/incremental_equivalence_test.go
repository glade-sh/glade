package typesys

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/packageartifact"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
)

func TestUpdateApexFilesMatchesFullBuildAcrossEditSequence(t *testing.T) {
	fixture := newIncrementalEquivalenceFixture(t)
	incremental := fixture.buildFresh(t)

	steps := []struct {
		name   string
		mutate func(t *testing.T) (changed, deleted []string)
	}{
		{
			name: "dependency class modify",
			mutate: func(t *testing.T) ([]string, []string) {
				writeFile(t, fixture.dependencyClass, "global class StageService { global static String modifiedValue() { return 'modified'; } }")
				return []string{fixture.dependencyClass}, nil
			},
		},
		{
			name: "local class modify",
			mutate: func(t *testing.T) ([]string, []string) {
				writeFile(t, fixture.localClass, "public class LocalService { public static String modifiedValue() { return 'modified'; } }")
				return []string{fixture.localClass}, nil
			},
		},
		{
			name: "local class add",
			mutate: func(t *testing.T) ([]string, []string) {
				writeFile(t, fixture.localAddedClass, "public class LocalAdded { public void run() {} }")
				fixture.localApexFiles = append(fixture.localApexFiles, fixture.localAddedClass)
				return []string{fixture.localAddedClass}, nil
			},
		},
		{
			name: "local class rename",
			mutate: func(t *testing.T) ([]string, []string) {
				fixture.renameFile(t, &fixture.localApexFiles, fixture.localAddedClass, fixture.localRenamedClass)
				writeFile(t, fixture.localRenamedClass, "public class LocalRenamed { public void run() {} }")
				return []string{fixture.localRenamedClass}, []string{fixture.localAddedClass}
			},
		},
		{
			name: "local class delete",
			mutate: func(t *testing.T) ([]string, []string) {
				fixture.deleteFile(t, &fixture.localApexFiles, fixture.localRenamedClass)
				return nil, []string{fixture.localRenamedClass}
			},
		},
		{
			name: "local trigger modify",
			mutate: func(t *testing.T) ([]string, []string) {
				writeFile(t, fixture.localTrigger, "trigger LocalTrigger on Account (before insert, before update) { Integer changed = Trigger.new.size(); }")
				return []string{fixture.localTrigger}, nil
			},
		},
		{
			name: "local trigger add",
			mutate: func(t *testing.T) ([]string, []string) {
				writeFile(t, fixture.localAddedTrigger, "trigger LocalAddedTrigger on Contact (after insert) {}")
				fixture.localApexFiles = append(fixture.localApexFiles, fixture.localAddedTrigger)
				return []string{fixture.localAddedTrigger}, nil
			},
		},
		{
			name: "local trigger rename",
			mutate: func(t *testing.T) ([]string, []string) {
				fixture.renameFile(t, &fixture.localApexFiles, fixture.localAddedTrigger, fixture.localRenamedTrigger)
				writeFile(t, fixture.localRenamedTrigger, "trigger LocalRenamedTrigger on Contact (after insert) {}")
				return []string{fixture.localRenamedTrigger}, []string{fixture.localAddedTrigger}
			},
		},
		{
			name: "local trigger delete",
			mutate: func(t *testing.T) ([]string, []string) {
				fixture.deleteFile(t, &fixture.localApexFiles, fixture.localRenamedTrigger)
				return nil, []string{fixture.localRenamedTrigger}
			},
		},
		{
			name: "dependency class add",
			mutate: func(t *testing.T) ([]string, []string) {
				writeFile(t, fixture.dependencyAddedClass, "global class StageAdded { global void run() {} }")
				fixture.dependencyApexFiles = append(fixture.dependencyApexFiles, fixture.dependencyAddedClass)
				return []string{fixture.dependencyAddedClass}, nil
			},
		},
		{
			name: "dependency class rename",
			mutate: func(t *testing.T) ([]string, []string) {
				fixture.renameFile(t, &fixture.dependencyApexFiles, fixture.dependencyAddedClass, fixture.dependencyRenamedClass)
				writeFile(t, fixture.dependencyRenamedClass, "global class StageRenamed { global void run() {} }")
				return []string{fixture.dependencyRenamedClass}, []string{fixture.dependencyAddedClass}
			},
		},
		{
			name: "dependency class delete",
			mutate: func(t *testing.T) ([]string, []string) {
				fixture.deleteFile(t, &fixture.dependencyApexFiles, fixture.dependencyRenamedClass)
				return nil, []string{fixture.dependencyRenamedClass}
			},
		},
		{
			name: "dependency trigger modify",
			mutate: func(t *testing.T) ([]string, []string) {
				writeFile(t, fixture.dependencyTrigger, "trigger StageTrigger on BasePkg__Ledger__c (before insert, after update) { BasePkg.StageService.modifiedValue(); }")
				return []string{fixture.dependencyTrigger}, nil
			},
		},
		{
			name: "dependency trigger add",
			mutate: func(t *testing.T) ([]string, []string) {
				writeFile(t, fixture.dependencyAddedTrigger, "trigger StageAddedTrigger on BasePkg__Ledger__c (after insert) {}")
				fixture.dependencyApexFiles = append(fixture.dependencyApexFiles, fixture.dependencyAddedTrigger)
				return []string{fixture.dependencyAddedTrigger}, nil
			},
		},
		{
			name: "dependency trigger rename",
			mutate: func(t *testing.T) ([]string, []string) {
				fixture.renameFile(t, &fixture.dependencyApexFiles, fixture.dependencyAddedTrigger, fixture.dependencyRenamedTrigger)
				writeFile(t, fixture.dependencyRenamedTrigger, "trigger StageRenamedTrigger on BasePkg__Ledger__c (after insert) {}")
				return []string{fixture.dependencyRenamedTrigger}, []string{fixture.dependencyAddedTrigger}
			},
		},
		{
			name: "dependency trigger delete",
			mutate: func(t *testing.T) ([]string, []string) {
				fixture.deleteFile(t, &fixture.dependencyApexFiles, fixture.dependencyRenamedTrigger)
				return nil, []string{fixture.dependencyRenamedTrigger}
			},
		},
	}

	for _, step := range steps {
		changed, deleted := step.mutate(t)
		incremental = UpdateApexFiles(incremental, changed, deleted)
		fresh := fixture.buildFresh(t)
		if mismatches := incrementalIndexMismatches(incremental, fresh); len(mismatches) > 0 {
			t.Errorf("%s: incremental index differs from full Build in fields %v", step.name, mismatches)
		}
	}
}

type incrementalEquivalenceFixture struct {
	consumerRoot             string
	dependencyRoot           string
	localClass               string
	localTrigger             string
	missingClass             string
	localAddedClass          string
	localRenamedClass        string
	localAddedTrigger        string
	localRenamedTrigger      string
	dependencyClass          string
	dependencyTrigger        string
	dependencyAddedClass     string
	dependencyRenamedClass   string
	dependencyAddedTrigger   string
	dependencyRenamedTrigger string
	localApexFiles           []string
	dependencyApexFiles      []string
}

func newIncrementalEquivalenceFixture(t *testing.T) *incrementalEquivalenceFixture {
	t.Helper()
	root := t.TempDir()
	consumerRoot := filepath.Join(root, "consumer")
	dependencyRoot := filepath.Join(root, "base-source")
	artifactPath := filepath.Join(root, "packages", "artifactpkg.glade-package.json")

	fixture := &incrementalEquivalenceFixture{
		consumerRoot:             consumerRoot,
		dependencyRoot:           dependencyRoot,
		localClass:               filepath.Join(consumerRoot, "force-app/main/default/classes/LocalService.cls"),
		localTrigger:             filepath.Join(consumerRoot, "force-app/main/default/triggers/LocalTrigger.trigger"),
		missingClass:             filepath.Join(consumerRoot, "force-app/main/default/classes/Missing.cls"),
		localAddedClass:          filepath.Join(consumerRoot, "force-app/main/default/classes/LocalAdded.cls"),
		localRenamedClass:        filepath.Join(consumerRoot, "force-app/main/default/classes/LocalRenamed.cls"),
		localAddedTrigger:        filepath.Join(consumerRoot, "force-app/main/default/triggers/LocalAddedTrigger.trigger"),
		localRenamedTrigger:      filepath.Join(consumerRoot, "force-app/main/default/triggers/LocalRenamedTrigger.trigger"),
		dependencyClass:          filepath.Join(dependencyRoot, "force-app/main/default/classes/StageService.cls"),
		dependencyTrigger:        filepath.Join(dependencyRoot, "force-app/main/default/triggers/StageTrigger.trigger"),
		dependencyAddedClass:     filepath.Join(dependencyRoot, "force-app/main/default/classes/StageAdded.cls"),
		dependencyRenamedClass:   filepath.Join(dependencyRoot, "force-app/main/default/classes/StageRenamed.cls"),
		dependencyAddedTrigger:   filepath.Join(dependencyRoot, "force-app/main/default/triggers/StageAddedTrigger.trigger"),
		dependencyRenamedTrigger: filepath.Join(dependencyRoot, "force-app/main/default/triggers/StageRenamedTrigger.trigger"),
	}
	fixture.localApexFiles = []string{fixture.localClass, fixture.localTrigger, fixture.missingClass}
	fixture.dependencyApexFiles = []string{fixture.dependencyClass, fixture.dependencyTrigger}

	writeFile(t, filepath.Join(consumerRoot, "sfdx-project.json"), `{
  "namespace": "localpkg",
  "sourceApiVersion": "63.0",
  "packageDirectories": [{"path": "force-app", "default": true}]
}`)
	writeFile(t, filepath.Join(consumerRoot, "glade.yml"), `project:
  namespaceRemaps: ["BasePkg:stagepkg"]
  managedPackageDependencies: ["artifactpkg:artifact:../packages/artifactpkg.glade-package.json:3.1.0", "stagepkg:../base-source:2.4.0"]
`)
	writeFile(t, filepath.Join(dependencyRoot, "sfdx-project.json"), `{
  "namespace": "BasePkg",
  "sourceApiVersion": "62.0",
  "packageDirectories": [{"path": "force-app", "default": true}]
}`)
	writeFile(t, fixture.localClass, "public class LocalService { public static String value() { return 'initial'; } }")
	writeFile(t, fixture.localTrigger, "trigger LocalTrigger on Account (before insert) {}")
	writeFile(t, fixture.dependencyClass, "global class StageService { global static String value() { return 'initial'; } }")
	writeFile(t, fixture.dependencyTrigger, "trigger StageTrigger on BasePkg__Ledger__c (before insert) { BasePkg.StageService.value(); }")

	writeFile(t, filepath.Join(consumerRoot, "force-app/main/default/objects/LocalLedger__c/LocalLedger__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Local Ledger</label><pluralLabel>Local Ledgers</pluralLabel><nameField><label>Local Ledger Name</label><type>Text</type></nameField><deploymentStatus>Deployed</deploymentStatus><sharingModel>ReadWrite</sharingModel></CustomObject>`)
	writeFile(t, filepath.Join(consumerRoot, "force-app/main/default/objects/LocalFeature__mdt/LocalFeature__mdt.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Local Feature</label><pluralLabel>Local Features</pluralLabel><visibility>Public</visibility></CustomObject>`)
	writeFile(t, filepath.Join(consumerRoot, "force-app/main/default/customMetadata/LocalFeature.Default.md-meta.xml"), `<CustomMetadata xmlns="http://soap.sforce.com/2006/04/metadata"><label>Default Feature</label><protected>false</protected><values><field>Enabled__c</field><value>true</value></values></CustomMetadata>`)

	artifact, err := packageartifact.BuildCaptured(packageartifact.BuildCapturedOptions{
		Namespace:        "artifactpkg",
		PackageName:      "Artifact Package",
		Version:          "3.1.0",
		SourceAPIVersion: "61.0",
		Capture: packageartifact.CaptureProvenance{
			Source:     "fixture",
			CapturedAt: time.Date(2026, time.July, 14, 12, 0, 0, 0, time.UTC),
		},
		ApexTypes: []packageartifact.ApexType{{
			Kind:       apexast.DeclarationClass,
			Name:       "ArtifactContract",
			File:       "artifact/ArtifactContract.cls",
			Namespace:  "artifactpkg",
			SourceRoot: "artifact",
			Version:    "3.1.0",
			Dependency: true,
			Modifiers:  []string{"global"},
		}},
		Objects: []schema.Object{{
			Name:   "artifactpkg__ArtifactLedger__c",
			Label:  "Artifact Ledger",
			Fields: []schema.Field{{Name: "artifactpkg__State__c", Type: "Text"}},
		}},
		CustomMetadataRecords: []schema.CustomMetadataRecord{{
			FullName:      "artifactpkg__ArtifactFeature__mdt.Default",
			ObjectName:    "artifactpkg__ArtifactFeature__mdt",
			DeveloperName: "Default",
			Label:         "Artifact Feature",
			File:          "artifact/ArtifactFeature.Default.md-meta.xml",
		}},
		LabelNames:          []string{"artifactpkg__Artifact_Label"},
		StaticResourceNames: []string{"artifactpkg__ArtifactAssets"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(artifactPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := packageartifact.WriteJSON(artifactPath, artifact); err != nil {
		t.Fatal(err)
	}

	initial := fixture.buildFresh(t)
	if initial.Project.Namespace != "localpkg" || initial.Project.SourceAPIVersion != "63.0" {
		t.Fatalf("project identity = %#v", initial.Project)
	}
	if len(initial.CustomMetadataRecords) < 2 || len(initial.CodeIntelSymbols) == 0 || len(initial.CodeIntelUses) == 0 {
		t.Fatalf("fixture did not produce rich metadata: customMetadata=%d symbols=%d uses=%d", len(initial.CustomMetadataRecords), len(initial.CodeIntelSymbols), len(initial.CodeIntelUses))
	}
	if len(initial.Dependencies) != 2 || len(initial.Diagnostics) == 0 {
		t.Fatalf("fixture did not produce dependencies and missing-file diagnostic: dependencies=%#v diagnostics=%#v", initial.Dependencies, initial.Diagnostics)
	}
	stageTypeFound := false
	for _, typ := range initial.Types {
		if typ.Name != "StageService" {
			continue
		}
		stageTypeFound = typ.Namespace == "stagepkg" && typ.SourceRoot == fixture.dependencyRoot && typ.Version == "2.4.0" && typ.Dependency && len(typ.SourceNamespaceRemaps) == 1 && typ.SourceNamespaceRemaps[0].From == "BasePkg" && typ.SourceNamespaceRemaps[0].To == "stagepkg"
	}
	if !stageTypeFound {
		t.Fatalf("fixture did not preserve source-backed dependency identity: %#v", initial.Types)
	}
	stageTriggerFound := false
	for _, trigger := range initial.Triggers {
		if trigger.Name != "StageTrigger" {
			continue
		}
		stageTriggerFound = trigger.Namespace == "stagepkg" && trigger.ObjectName == "stagepkg__Ledger__c" && trigger.Dependency && len(trigger.SourceNamespaceRemaps) == 1 && trigger.SourceNamespaceRemaps[0].From == "BasePkg" && trigger.SourceNamespaceRemaps[0].To == "stagepkg"
	}
	if !stageTriggerFound {
		t.Fatalf("fixture did not preserve source-backed trigger identity: %#v", initial.Triggers)
	}
	return fixture
}

func (f *incrementalEquivalenceFixture) buildFresh(t *testing.T) Index {
	t.Helper()
	p, err := project.Load(f.consumerRoot)
	if err != nil {
		t.Fatal(err)
	}
	p.ApexFiles = append([]string(nil), f.localApexFiles...)
	artifactLoaded := false
	stageLoaded := false
	for i := range p.ManagedPackageDependencies {
		dep := &p.ManagedPackageDependencies[i]
		switch dep.Namespace {
		case "artifactpkg":
			artifactLoaded = dep.Status == "loaded" && dep.ArtifactPath != ""
		case "stagepkg":
			stageLoaded = dep.Status == "loaded" && dep.Project != nil
			if dep.Project != nil {
				dep.Project.ApexFiles = append([]string(nil), f.dependencyApexFiles...)
			}
		}
	}
	if !artifactLoaded || !stageLoaded {
		t.Fatalf("project.Load dependencies artifact=%t stage=%t: %#v", artifactLoaded, stageLoaded, p.ManagedPackageDependencies)
	}
	s, err := schema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	return Build(p, s)
}

func (f *incrementalEquivalenceFixture) renameFile(t *testing.T, files *[]string, oldPath, newPath string) {
	t.Helper()
	if err := os.Rename(oldPath, newPath); err != nil {
		t.Fatal(err)
	}
	for i, path := range *files {
		if path == oldPath {
			(*files)[i] = newPath
			return
		}
	}
	t.Fatalf("rename source %q missing from explicit Apex file list", oldPath)
}

func (f *incrementalEquivalenceFixture) deleteFile(t *testing.T, files *[]string, path string) {
	t.Helper()
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	for i, candidate := range *files {
		if candidate == path {
			*files = append((*files)[:i], (*files)[i+1:]...)
			return
		}
	}
	t.Fatalf("deleted path %q missing from explicit Apex file list", path)
}

func incrementalIndexMismatches(got, want Index) []string {
	var mismatches []string
	if !reflect.DeepEqual(got.Project, want.Project) {
		mismatches = append(mismatches, "Project")
	}
	if !reflect.DeepEqual(got.Types, want.Types) {
		mismatches = append(mismatches, "Types")
	}
	if !reflect.DeepEqual(got.Triggers, want.Triggers) {
		mismatches = append(mismatches, "Triggers")
	}
	if !reflect.DeepEqual(got.Objects, want.Objects) {
		mismatches = append(mismatches, "Objects")
	}
	if !reflect.DeepEqual(got.CustomMetadataRecords, want.CustomMetadataRecords) {
		mismatches = append(mismatches, "CustomMetadataRecords")
	}
	if !reflect.DeepEqual(got.CodeIntelSymbols, want.CodeIntelSymbols) {
		mismatches = append(mismatches, "CodeIntelSymbols")
	}
	if !reflect.DeepEqual(got.CodeIntelUses, want.CodeIntelUses) {
		mismatches = append(mismatches, "CodeIntelUses")
	}
	if !reflect.DeepEqual(got.Dependencies, want.Dependencies) {
		mismatches = append(mismatches, "Dependencies")
	}
	if !reflect.DeepEqual(got.Diagnostics, want.Diagnostics) {
		mismatches = append(mismatches, "Diagnostics")
	}
	return mismatches
}
