package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestNormalizeTextWrites(t *testing.T) {
	definition := storage.ObjectDefinition{APIName: "Account", Fields: map[string]storage.Field{
		"Name":        {APIName: "Name", Type: storage.FieldString},
		"Site":        {APIName: "Site", Type: storage.FieldString, DisplayType: "STRING"},
		"Description": {APIName: "Description", Type: storage.FieldString, DisplayType: "TEXTAREA"},
		"Phone":       {APIName: "Phone", Type: storage.FieldString, DisplayType: "PHONE"},
	}}
	record := storage.Record{Object: "Account", Fields: map[string]storage.Value{
		"Name":        storage.StringValue(" \tpadded name \t"),
		"Site":        storage.StringValue("  \t "),
		"Description": storage.StringValue("  keep  "),
		"Phone":       storage.StringValue(" 555 "),
	}}
	record.Fields["Industry"] = storage.StringValue("\r\nkeep\v")
	definition.Fields["Industry"] = storage.Field{APIName: "Industry", Type: storage.FieldString}
	NormalizeTextWrites(definition, "", &record, "")
	if got := record.Fields["Industry"].String; got != "\r\nkeep\v" {
		t.Fatalf("other control characters changed: %q", got)
	}
	if got := record.Fields["Name"].String; got != "padded name" {
		t.Fatalf("Name = %q; want trimmed", got)
	}
	if _, present := record.Fields["Site"]; present || !record.ExplicitNulls["Site"] {
		t.Fatalf("blank Site should be an explicit null: %v %v", record.Fields, record.ExplicitNulls)
	}
	if got := record.Fields["Description"].String; got != "  keep  " {
		t.Fatalf("text area changed: %q", got)
	}
	if got := record.Fields["Phone"].String; got != " 555 " {
		t.Fatalf("phone changed: %q", got)
	}
}
