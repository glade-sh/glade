package sema

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/schema"
)

func TestPublicCorpusAllowsStaticMethodCallThroughInstance(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "StaticThroughInstance.cls"), `
public class StaticThroughInstance {
  public List<Contact> run(Set<Id> accountIds) {
    ContactSelector contactSelector = new ContactSelector();
    List<Contact> contacts = contactSelector.getContactAddressFieldsForContactAccountsIn(accountIds);
    return contacts;
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "ContactSelector.cls"), `
public class ContactSelector {
  public static List<Contact> getContactAddressFieldsForContactAccountsIn(Set<Id> householdAccountIds) {
    return new List<Contact>();
  }
}
`)
	result := analyzePublicCorpusFiles(t, root, "StaticThroughInstance.cls", "ContactSelector.cls")
	assertNoDiagnosticContaining(t, result, "GLADESEMA027", "static method called through an instance")
}

func TestPublicCorpusAllowsDocumentInstanceFieldAssignmentsRegardlessOfWarmup(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		files []string
	}{
		{"writer alone", []string{"DocumentWriter.cls"}},
		{"warmup before writer", []string{"Document.cls", "DocumentWriter.cls"}},
		{"writer before warmup", []string{"DocumentWriter.cls", "Document.cls"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeSemaFile(t, filepath.Join(root, "DocumentWriter.cls"), `
public class DocumentWriter {
  public static void write(Id folderId, Blob body) {
    Document document = new Document();
    document.FolderId = folderId;
    document.Body = body;
  }
}
`)
			writeSemaFile(t, filepath.Join(root, "Document.cls"), `
public class Document {
  public String description;
}
`)
			result := analyzePublicCorpusWithSchema(t, root, schema.Schema{Objects: []schema.Object{{Name: "Archive__c"}}}, tc.files...)
			assertNoDiagnosticContaining(t, result, "GLADESEMA027", "static fields cannot be accessed through an instance")
		})
	}
}
