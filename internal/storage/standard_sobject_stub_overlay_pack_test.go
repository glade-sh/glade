package storage

import (
	"compress/gzip"
	"encoding/csv"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestStandardSObjectStubEmbeddedPackMatchesLegacyLiteralData(t *testing.T) {
	generated, err := os.ReadFile("standard_sobject_stub_overlay_generated.go")
	if err != nil {
		t.Fatal(err)
	}
	entries := standardSObjectStubGeneratedMapLiteralEntries(generated)
	t.Logf("generated standard sObject map-literal initializer entries = %d", entries)
	if entries != 0 {
		t.Errorf("generated Go still contains %d standard sObject map-literal initialization entries", entries)
	}
	packInfo, err := os.Stat("standard_sobject_stub_overlay.pack")
	if err != nil {
		t.Errorf("generated embedded pack is unavailable: %v", err)
	} else if packInfo.Size() == 0 {
		t.Error("generated embedded pack is empty")
	}

	legacyFile, err := os.Open("testdata/standard_sobject_stub_native/legacy.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	reader, err := gzip.NewReader(legacyFile)
	if err != nil {
		_ = legacyFile.Close()
		t.Fatal(err)
	}
	legacyJSON, readErr := io.ReadAll(reader)
	closeErr := reader.Close()
	fileCloseErr := legacyFile.Close()
	if readErr != nil {
		t.Fatal(readErr)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	if fileCloseErr != nil {
		t.Fatal(fileCloseErr)
	}

	var want struct {
		Objects        map[string]standardSObjectStubObjectInfo `json:"objects"`
		Fields         map[string]map[string]Field              `json:"fields"`
		ReadOnlyFields map[string][]string                      `json:"readOnlyFields"`
		Relationships  map[string][]Relationship                `json:"relationships"`
	}
	if err := json.Unmarshal(legacyJSON, &want); err != nil {
		t.Fatal(err)
	}

	gotObjects := make(map[string]standardSObjectStubObjectInfo, len(want.Objects))
	for name := range want.Objects {
		value, ok := standardSObjectStubObjectInfoFor(name)
		if !ok {
			t.Errorf("object info missing for %s", name)
			continue
		}
		gotObjects[name] = value
	}
	if !reflect.DeepEqual(gotObjects, want.Objects) {
		t.Error("decoded object info differs from the old generated literals")
	}

	gotFields := make(map[string]map[string]Field, len(want.Fields))
	for name := range want.Fields {
		value, ok := standardSObjectStubFieldsFor(name)
		if !ok {
			t.Errorf("fields missing for %s", name)
			continue
		}
		gotFields[name] = value
	}
	if !reflect.DeepEqual(gotFields, want.Fields) {
		t.Error("decoded fields differ from the old generated literals")
	}

	gotReadOnly := make(map[string][]string, len(want.ReadOnlyFields))
	for name := range want.ReadOnlyFields {
		value, ok := standardSObjectStubReadOnlyFieldsFor(name)
		if !ok {
			t.Errorf("read-only fields missing for %s", name)
			continue
		}
		gotReadOnly[name] = value
	}
	if !reflect.DeepEqual(gotReadOnly, want.ReadOnlyFields) {
		t.Error("decoded read-only fields differ from the old generated literals")
	}

	gotRelationships := make(map[string][]Relationship, len(want.Relationships))
	for name := range want.Relationships {
		value, ok := standardSObjectStubRelationshipsFor(name)
		if !ok {
			t.Errorf("relationships missing for %s", name)
			continue
		}
		gotRelationships[name] = value
	}
	if !reflect.DeepEqual(gotRelationships, want.Relationships) {
		t.Error("decoded relationships differ from the old generated literals")
	}

	wantNames := make([]string, 0, len(want.Fields))
	for name := range want.Fields {
		wantNames = append(wantNames, name)
	}
	gotNames := standardSObjectStubNames()
	sort.Strings(wantNames)
	sort.Strings(gotNames)
	if !reflect.DeepEqual(gotNames, wantNames) {
		t.Error("object names differ from the old generated literals")
	}
	if _, ok := standardSObjectStubObjectInfoFor("CodexUnknownStandardObject"); ok {
		t.Error("unknown object unexpectedly resolved")
	}
}

func TestStandardSObjectStubMatchesNativeRows(t *testing.T) {
	file, err := os.Open("testdata/standard_sobject_stub_native/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.Comma = '\t'
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) < 2 {
		t.Fatal("native row table is empty")
	}
	header := make(map[string]int, len(records[0]))
	for index, name := range records[0] {
		header[name] = index
	}
	value := func(record []string, name string) string {
		index, ok := header[name]
		if !ok || index >= len(record) {
			return ""
		}
		return record[index]
	}
	versions := make(map[string]bool)
	for line, record := range records[1:] {
		apiVersion := value(record, "apiVersion")
		objectName := value(record, "object")
		fieldName := value(record, "field")
		versions[apiVersion] = true
		switch value(record, "kind") {
		case "object":
			got, ok := standardSObjectStubObjectInfoFor(objectName)
			if !ok || got.Label != value(record, "label") || got.PluralLabel != value(record, "pluralLabel") {
				t.Errorf("native row %d API %s object %s differs: got %+v, ok=%t", line+2, apiVersion, objectName, got, ok)
			}
		case "field":
			fields, ok := standardSObjectStubFieldsFor(objectName)
			got, fieldOK := fields[fieldName]
			if !ok || !fieldOK {
				t.Errorf("native row %d API %s field %s.%s is missing", line+2, apiVersion, objectName, fieldName)
				continue
			}
			referenceTo := append([]string(nil), got.ReferenceTo...)
			sort.Strings(referenceTo)
			if got.DisplayType != value(record, "type") || got.Label != value(record, "label") ||
				strings.Join(referenceTo, ";") != value(record, "referenceTo") || got.RelationshipName != value(record, "relationshipName") {
				t.Errorf("native row %d API %s field %s.%s differs: got type=%q label=%q references=%q relationship=%q",
					line+2, apiVersion, objectName, fieldName, got.DisplayType, got.Label, strings.Join(referenceTo, ";"), got.RelationshipName)
			}
		case "childRelationship":
			relationships, ok := standardSObjectStubRelationshipsFor(objectName)
			matched := false
			for _, relationship := range relationships {
				if relationship.Field != fieldName || relationship.ChildRelationship != value(record, "relationshipName") {
					continue
				}
				for _, parent := range relationship.ParentObjects {
					if parent == value(record, "parentObject") {
						matched = true
					}
				}
			}
			if !ok || !matched {
				t.Errorf("native row %d API %s child relationship %s.%s -> %s differs", line+2, apiVersion, objectName, fieldName, value(record, "relationshipName"))
			}
		default:
			t.Errorf("native row %d has unsupported kind %q", line+2, value(record, "kind"))
		}
	}
	if !versions["67.0"] || !versions["62.0"] {
		t.Errorf("native row APIs = %v, want 62.0 and 67.0", versions)
	}
}

func standardSObjectStubGeneratedMapLiteralEntries(source []byte) int {
	file, err := parser.ParseFile(token.NewFileSet(), "standard_sobject_stub_overlay_generated.go", source, 0)
	if err != nil {
		return -1
	}
	targets := map[string]bool{
		"standardSObjectStubObjectData":        true,
		"standardSObjectStubFieldData":         true,
		"standardSObjectStubReadOnlyFieldData": true,
		"standardSObjectStubRelationshipData":  true,
	}
	entries := 0
	for _, declaration := range file.Decls {
		general, ok := declaration.(*ast.GenDecl)
		if !ok || general.Tok != token.VAR {
			continue
		}
		for _, specification := range general.Specs {
			value, ok := specification.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for index, name := range value.Names {
				if !targets[name.Name] || index >= len(value.Values) {
					continue
				}
				entries += countStandardSObjectStubMapLiteralEntries(value.Values[index], false)
			}
		}
	}
	return entries
}

func countStandardSObjectStubMapLiteralEntries(expression ast.Expr, isElidedMap bool) int {
	literal, ok := expression.(*ast.CompositeLit)
	if !ok {
		return 0
	}
	mapType, isTypedMap := literal.Type.(*ast.MapType)
	if !isTypedMap {
		if isElidedMap {
			return len(literal.Elts)
		}
		return 0
	}
	entries := len(literal.Elts)
	valueIsMap := false
	if _, valueIsMap = mapType.Value.(*ast.MapType); valueIsMap {
		for _, element := range literal.Elts {
			keyValue, ok := element.(*ast.KeyValueExpr)
			if ok {
				entries += countStandardSObjectStubMapLiteralEntries(keyValue.Value, true)
			}
		}
	}
	return entries
}
