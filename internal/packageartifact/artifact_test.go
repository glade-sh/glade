package packageartifact

import (
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
)

func TestBuildAppliesInstalledNamespaceToCustomSchema(t *testing.T) {
	artifact, err := Build("pkg", "1.0", project.Project{Root: t.TempDir()}, schema.Schema{Objects: []schema.Object{
		{
			Name: "CartItemLine__c",
			Fields: []schema.Field{
				{Name: "Product__c", Type: "reference", ReferenceTo: []string{"Membership__c"}},
				{Name: "Name", Type: "string"},
			},
		},
		{
			Name:   "Account",
			Fields: []schema.Field{{Name: "Name", Type: "string"}},
		},
	}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if artifact.Objects[0].Name != "pkg__CartItemLine__c" {
		t.Fatalf("object name = %q", artifact.Objects[0].Name)
	}
	if artifact.Objects[0].Fields[0].Name != "pkg__Product__c" {
		t.Fatalf("field name = %q", artifact.Objects[0].Fields[0].Name)
	}
	if artifact.Objects[0].Fields[0].ReferenceTo[0] != "pkg__Membership__c" {
		t.Fatalf("referenceTo = %#v", artifact.Objects[0].Fields[0].ReferenceTo)
	}
	if artifact.Objects[0].Fields[1].Name != "Name" {
		t.Fatalf("standard field name = %q", artifact.Objects[0].Fields[1].Name)
	}
	if artifact.Objects[1].Name != "Account" {
		t.Fatalf("standard object name = %q", artifact.Objects[1].Name)
	}
}

func TestCompareIgnoresLocalApexFilePaths(t *testing.T) {
	from := Artifact{
		Namespace: "pkg",
		ApexTypes: []ApexType{{
			Name:       "Address",
			Namespace:  "pkg",
			File:       "/tmp/one/force-app/classes/Address.cls",
			SourceRoot: "/tmp/one",
			Members:    []ApexMember{{Name: "format"}},
		}},
	}
	to := Artifact{
		Namespace: "pkg",
		ApexTypes: []ApexType{{
			Name:       "Address",
			Namespace:  "pkg",
			File:       "/tmp/two/force-app/classes/Address.cls",
			SourceRoot: "/tmp/two",
			Members:    []ApexMember{{Name: "format"}},
		}},
	}

	diff := Compare(from, to)
	if diff.ChangedTypes != 0 {
		t.Fatalf("changedTypes = %d, want 0; changed names=%#v", diff.ChangedTypes, diff.ChangedTypeNames)
	}
}

func TestCompareReportsChangedObjects(t *testing.T) {
	from := Artifact{
		Namespace:  "pkg",
		SourceHash: "same",
		Objects: []schema.Object{{
			Name:   "pkg__Thing__c",
			Fields: []schema.Field{{Name: "Name", Type: "string"}},
		}},
	}
	to := Artifact{
		Namespace:  "pkg",
		SourceHash: "same",
		Objects: []schema.Object{{
			Name:   "pkg__Thing__c",
			Fields: []schema.Field{{Name: "Name", Type: "textarea"}},
		}},
	}

	diff := Compare(from, to)
	if !diff.Changed {
		t.Fatal("changed = false, want true")
	}
	if diff.ChangedObjects != 1 || len(diff.ChangedObjectNames) != 1 || diff.ChangedObjectNames[0] != "pkg__Thing__c" {
		t.Fatalf("changed objects = %d %#v", diff.ChangedObjects, diff.ChangedObjectNames)
	}
}
