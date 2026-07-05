package sema

import (
	"path/filepath"
	"testing"
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
