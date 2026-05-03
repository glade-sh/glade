package apexdocs

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildInventoryParsesSystemMethodPage(t *testing.T) {
	root := t.TempDir()
	writeDoc(t, filepath.Join(root, "apex_methods_system_string.md"), `# String Class

## Namespace
[System](./apex_namespace_System.md)

## String Methods
### contains(substring)
Returns true if this string contains the specified substring.

### trim()
Removes leading and trailing white space.

### Example: Trim a value
String s = ' hi ';
`)

	inv, err := BuildInventory(root)
	if err != nil {
		t.Fatal(err)
	}
	if inv.SchemaVersion != InventorySchemaVersion {
		t.Fatalf("schema version = %d", inv.SchemaVersion)
	}
	if inv.TotalFiles != 1 || inv.TotalMembers != 2 {
		t.Fatalf("summary = files %d members %d", inv.TotalFiles, inv.TotalMembers)
	}
	doc := inv.Documents[0]
	if doc.SourcePath != "apex_methods_system_string.md" || doc.Kind != "class" || doc.Namespace != "System" || doc.Name != "String" {
		t.Fatalf("doc metadata = %#v", doc)
	}
	if got := doc.Members[0].Signature; got != "contains(substring)" {
		t.Fatalf("first member = %q", got)
	}
	if len(doc.Examples) != 1 || doc.Examples[0].Heading != "Example: Trim a value" {
		t.Fatalf("examples = %#v", doc.Examples)
	}
}

func TestBuildInventoryParsesProductNamespaceShapes(t *testing.T) {
	root := t.TempDir()
	writeDoc(t, filepath.Join(root, "apex_class_Metadata_ConsoleComponent.md"), `# ConsoleComponent Class

## Namespace
[Metadata](./apex_namespace_Metadata.md)

## ConsoleComponent Properties
### label
The component label.
`)
	writeDoc(t, filepath.Join(root, "apex_connectapi_output_ChatterConversationPage.md"), `# ChatterConversationPage

## Properties
### conversations
Page values.
`)

	inv, err := BuildInventory(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(inv.Documents) != 2 {
		t.Fatalf("documents = %d", len(inv.Documents))
	}
	if inv.Documents[0].Namespace != "Metadata" || inv.Documents[0].Kind != "class" {
		t.Fatalf("metadata doc = %#v", inv.Documents[0])
	}
	if inv.Documents[1].Namespace != "ConnectApi" || inv.Documents[1].Kind != "output" {
		t.Fatalf("connectapi doc = %#v", inv.Documents[1])
	}
}

func TestWriteJSONIsStable(t *testing.T) {
	root := t.TempDir()
	writeDoc(t, filepath.Join(root, "apex_methods_system_list.md"), `# List Class

## List Methods
### add(listElement)
Adds an element.
`)
	inv, err := BuildInventory(root)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := WriteJSON(&out, inv); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"sourcePath": "apex_methods_system_list.md"`) {
		t.Fatalf("json = %q", out.String())
	}
	var decoded Inventory
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Documents[0].Members[0].Name != "add" {
		t.Fatalf("decoded = %#v", decoded)
	}
}

func TestDiffInventories(t *testing.T) {
	oldInv := Inventory{Documents: []Document{{
		SourcePath: "a.md",
		Kind:       "class",
		Namespace:  "System",
		Name:       "A",
		Title:      "A Class",
		Members:    []Member{{Kind: "method", Name: "one", Signature: "one()"}},
	}}}
	newInv := Inventory{Documents: []Document{{
		SourcePath: "a.md",
		Kind:       "class",
		Namespace:  "System",
		Name:       "A",
		Title:      "A Class",
		Members:    []Member{{Kind: "method", Name: "two", Signature: "two()"}},
	}, {
		SourcePath: "b.md",
		Kind:       "class",
		Name:       "B",
	}}}
	diff := DiffInventories(oldInv, newInv)
	if len(diff.AddedDocuments) != 1 || diff.AddedDocuments[0] != "b.md" {
		t.Fatalf("added docs = %#v", diff.AddedDocuments)
	}
	if len(diff.ChangedDocuments) != 1 {
		t.Fatalf("changed docs = %#v", diff.ChangedDocuments)
	}
	changed := diff.ChangedDocuments[0]
	if changed.AddedMembers[0] != "method|two|two()" || changed.RemovedMembers[0] != "method|one|one()" {
		t.Fatalf("changed = %#v", changed)
	}
}

func writeDoc(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
