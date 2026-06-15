package codeintel_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/codeintel"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestSchemaUsesResolveSObjectConstructionFieldsSOQLDMLAndMetadataTokens(t *testing.T) {
	root := t.TempDir()
	classPath := filepath.Join(root, "force-app/main/default/classes/Probe.cls")
	triggerPath := filepath.Join(root, "force-app/main/default/triggers/AccountTrigger.trigger")
	writeTestFile(t, classPath, `public class Probe {
  public void run() {
    Account a = new Account(Name = 'Acme');
    a.Name = 'Other';
    List<Account> rows = [SELECT Id, Name FROM Account WHERE Owner.Name != null];
    insert a;
    Object token = Schema.SObjectType.Account.fields.Name;
    Object bareToken = Schema.SObjectType.User;
    Object objectToken = Contact.SObjectType;
  }
}
`)
	writeTestFile(t, triggerPath, `trigger AccountTrigger on Account (before insert) {
}
`)

	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{classPath, triggerPath},
	}, schema.Schema{Objects: testSchemaObjects()})
	if len(index.Diagnostics) > 0 {
		t.Fatalf("index diagnostics = %#v", index.Diagnostics)
	}

	graph := codeintel.BuildSchemaUses(index)

	assertUse(t, graph, codeintel.SObjectID("Account"), codeintel.UseConstruct, "Account")
	assertUse(t, graph, codeintel.SObjectFieldID("Account", "Name"), codeintel.UseWrite, "Name")
	assertUse(t, graph, codeintel.SObjectID("Account"), codeintel.UseQuery, "Account")
	assertUse(t, graph, codeintel.SObjectFieldID("Account", "Id"), codeintel.UseQuery, "Id")
	assertUse(t, graph, codeintel.SObjectFieldID("Account", "Name"), codeintel.UseQuery, "Name")
	assertUse(t, graph, codeintel.SObjectID("Account"), codeintel.UseMutate, "Account")
	assertUse(t, graph, codeintel.SObjectID("Account"), codeintel.UseMetadata, "Account")
	assertUse(t, graph, codeintel.SObjectFieldID("Account", "Name"), codeintel.UseMetadata, "Name")
	assertUse(t, graph, codeintel.SObjectID("Contact"), codeintel.UseMetadata, "Contact")
	assertUse(t, graph, codeintel.SObjectID("User"), codeintel.UseMetadata, "User")
	assertUseCount(t, graph, codeintel.SObjectID("Account"), codeintel.UseRead, "Account", 1)
	assertUseMetadataCount(t, graph, codeintel.SObjectID("Account"), codeintel.UseMetadata, "Account", "source", "schema_token", 1)

	triggerUse := assertUseWithMetadata(t, graph, codeintel.SObjectID("Account"), codeintel.UseMetadata, "Account", "source", "trigger")
	if triggerUse.Metadata["source"] != "trigger" {
		t.Fatalf("trigger object use metadata = %#v, want source=trigger", triggerUse.Metadata)
	}

	relationshipUse := assertUse(t, graph, codeintel.SObjectFieldID("User", "Name"), codeintel.UseQuery, "Owner.Name")
	if relationshipUse.Metadata["relationshipPath"] != "Owner.Name" ||
		relationshipUse.Metadata["relationshipObject"] != "User" ||
		relationshipUse.Metadata["rootObject"] != "Account" {
		t.Fatalf("relationship metadata = %#v", relationshipUse.Metadata)
	}
}

func TestSchemaUsesResolveChildSubqueryFields(t *testing.T) {
	root := t.TempDir()
	classPath := filepath.Join(root, "force-app/main/default/classes/Probe.cls")
	writeTestFile(t, classPath, `public class Probe {
  public void run() {
    List<Account> rows = [SELECT Id, (SELECT LastName FROM Contacts) FROM Account];
  }
}
`)

	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{classPath},
	}, schema.Schema{Objects: testSchemaObjects()})
	if len(index.Diagnostics) > 0 {
		t.Fatalf("index diagnostics = %#v", index.Diagnostics)
	}

	graph := codeintel.BuildSchemaUses(index)

	childUse := assertUse(t, graph, codeintel.SObjectFieldID("Contact", "LastName"), codeintel.UseQuery, "LastName")
	if childUse.Metadata["childRelationship"] != "Contacts" ||
		childUse.Metadata["parentObject"] != "Account" ||
		childUse.Metadata["object"] != "Contact" {
		t.Fatalf("child query metadata = %#v", childUse.Metadata)
	}
}

func assertUse(t *testing.T, graph codeintel.Graph, id codeintel.SymbolID, kind codeintel.UseKind, name string) codeintel.Use {
	t.Helper()
	for _, use := range graph.Uses {
		if use.SymbolID == id && use.Kind == kind && use.Name == name && use.Resolved {
			return use
		}
	}
	t.Fatalf("missing use id=%s kind=%s name=%s in %#v", id, kind, name, graph.Uses)
	return codeintel.Use{}
}

func assertUseWithMetadata(t *testing.T, graph codeintel.Graph, id codeintel.SymbolID, kind codeintel.UseKind, name, key, value string) codeintel.Use {
	t.Helper()
	for _, use := range graph.Uses {
		if use.SymbolID == id && use.Kind == kind && use.Name == name && use.Resolved && use.Metadata[key] == value {
			return use
		}
	}
	t.Fatalf("missing use id=%s kind=%s name=%s metadata %s=%s in %#v", id, kind, name, key, value, graph.Uses)
	return codeintel.Use{}
}

func assertUseCount(t *testing.T, graph codeintel.Graph, id codeintel.SymbolID, kind codeintel.UseKind, name string, want int) {
	t.Helper()
	got := 0
	for _, use := range graph.Uses {
		if use.SymbolID == id && use.Kind == kind && use.Name == name && use.Resolved {
			got++
		}
	}
	if got != want {
		t.Fatalf("use count id=%s kind=%s name=%s = %d, want %d in %#v", id, kind, name, got, want, graph.Uses)
	}
}

func assertUseMetadataCount(t *testing.T, graph codeintel.Graph, id codeintel.SymbolID, kind codeintel.UseKind, name, key, value string, want int) {
	t.Helper()
	got := 0
	for _, use := range graph.Uses {
		if use.SymbolID == id && use.Kind == kind && use.Name == name && use.Resolved && use.Metadata[key] == value {
			got++
		}
	}
	if got != want {
		t.Fatalf("metadata use count id=%s kind=%s name=%s %s=%s = %d, want %d in %#v", id, kind, name, key, value, got, want, graph.Uses)
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func testSchemaObjects() []schema.Object {
	return []schema.Object{
		{
			Name: "Account",
			Fields: []schema.Field{
				{Name: "Id", Type: "Id"},
				{Name: "Name", Type: "String"},
				{Name: "OwnerId", Type: "Lookup", ReferenceTo: []string{"User"}, RelationshipName: "Owner"},
			},
		},
		{
			Name: "User",
			Fields: []schema.Field{
				{Name: "Id", Type: "Id"},
				{Name: "Name", Type: "String"},
			},
		},
		{
			Name: "Contact",
			Fields: []schema.Field{
				{Name: "Id", Type: "Id"},
				{Name: "LastName", Type: "String"},
				{Name: "AccountId", Type: "Lookup", ReferenceTo: []string{"Account"}, RelationshipName: "Account", ChildRelationshipName: "Contacts"},
			},
		},
	}
}
