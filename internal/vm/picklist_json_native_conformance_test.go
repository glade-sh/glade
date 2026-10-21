package vm

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sobject"
	"github.com/glade-sh/glade/internal/storage"
)

type picklistJSONNativeFile struct{ Path, Source, SHA256 string }
type picklistJSONNativeFixture struct {
	Fixtures []struct {
		ID    string
		Files []picklistJSONNativeFile
	}
	Captures []struct {
		ID, Fixture, SourceAPI, TransportAPI, Context                                       string
		OrgIDHash                                                                           string `json:"orgIdSHA256"`
		DeploymentID, DeploymentStatus, HelperPath, ObserverPath                            string
		AnonymousSource, AnonymousSourceSHA256, RawResponseSHA256, AnswerJSON, AnswerSHA256 string
	}
}

type picklistJSONNativeEntry struct {
	Index       *int            `json:"index"`
	RawJSON     string          `json:"rawJSON"`
	HasValidFor bool            `json:"hasValidFor"`
	ValidFor    json.RawMessage `json:"validFor"`
	Hex         *string         `json:"hex"`
	Value       json.RawMessage `json:"value"`
	Active      json.RawMessage `json:"active"`
}

func loadPicklistJSONNativeFixture(t *testing.T) picklistJSONNativeFixture {
	t.Helper()
	data, err := os.ReadFile("testdata/picklist_json_observations.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture picklistJSONNativeFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func picklistJSONNativeFiles(t *testing.T, fixture picklistJSONNativeFixture, id string) map[string]picklistJSONNativeFile {
	t.Helper()
	for _, group := range fixture.Fixtures {
		if group.ID != id {
			continue
		}
		files := make(map[string]picklistJSONNativeFile)
		for _, file := range group.Files {
			if _, duplicate := files[file.Path]; duplicate {
				t.Fatalf("duplicate fixture file %s", file.Path)
			}
			files[file.Path] = file
		}
		return files
	}
	t.Fatalf("unknown metadata fixture %q", id)
	return nil
}

func picklistJSONNativeDigest(source string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(source)))
}

func picklistJSONNativeAnswer(t *testing.T, answer string) map[string]json.RawMessage {
	t.Helper()
	var observed map[string]json.RawMessage
	if err := json.Unmarshal([]byte(answer), &observed); err != nil {
		t.Fatal(err)
	}
	if observed == nil {
		t.Fatal("native answer is not an object")
	}
	return observed
}

func picklistJSONNativeEntries(t *testing.T, raw json.RawMessage) []picklistJSONNativeEntry {
	t.Helper()
	var entries []picklistJSONNativeEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		var encoded map[string]json.RawMessage
		if err := json.Unmarshal([]byte(entry.RawJSON), &encoded); err != nil {
			t.Fatal(err)
		}
		observed, has := encoded["validFor"]
		if has != entry.HasValidFor {
			t.Fatal("native validFor presence disagrees with rawJSON")
		}
		if has && !bytes.Equal(bytes.TrimSpace(observed), bytes.TrimSpace(entry.ValidFor)) {
			t.Fatal("native validFor value disagrees with rawJSON")
		}
		for key, direct := range map[string]json.RawMessage{"value": entry.Value, "active": entry.Active} {
			if len(direct) > 0 && !bytes.Equal(bytes.TrimSpace(encoded[key]), bytes.TrimSpace(direct)) {
				t.Fatalf("native %s disagrees with rawJSON", key)
			}
		}
		for _, key := range []string{"value", "label", "active", "defaultValue"} {
			if _, ok := encoded[key]; !ok {
				t.Fatalf("native rawJSON lacks %s", key)
			}
		}
	}
	return entries
}

func TestPicklistJSONNativeFixtureIntegrity(t *testing.T) {
	fixture := loadPicklistJSONNativeFixture(t)
	if len(fixture.Fixtures) != 3 || len(fixture.Captures) != 10 {
		t.Fatal("native fixture scope changed")
	}
	validHash := func(hash string) bool {
		decoded, err := hex.DecodeString(hash)
		return err == nil && len(decoded) == 32
	}
	fixtureIDs := make(map[string]bool)
	for _, group := range fixture.Fixtures {
		if group.ID == "" || fixtureIDs[group.ID] {
			t.Fatal("missing or duplicate fixture identity")
		}
		fixtureIDs[group.ID] = true
		if len(group.Files) == 0 {
			t.Fatal("empty metadata source fixture")
		}
		for _, file := range group.Files {
			clean := filepath.Clean(filepath.FromSlash(file.Path))
			if file.Path == "" || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
				t.Fatal("unsafe source fixture path")
			}
			if picklistJSONNativeDigest(file.Source) != file.SHA256 {
				t.Fatalf("source digest changed: %s", file.Path)
			}
		}
	}
	captureIDs := make(map[string]bool)
	for _, capture := range fixture.Captures {
		if capture.ID == "" || captureIDs[capture.ID] {
			t.Fatal("missing or duplicate native capture")
		}
		captureIDs[capture.ID] = true
		if capture.SourceAPI != "61.0" && capture.SourceAPI != "62.0" && capture.SourceAPI != "67.0" {
			t.Fatal("unobserved source API")
		}
		if !validHash(capture.OrgIDHash) || !validHash(capture.RawResponseSHA256) || capture.DeploymentID == "" || capture.DeploymentStatus != "Succeeded" {
			t.Fatal("incomplete native provenance")
		}
		if picklistJSONNativeDigest(capture.AnswerJSON) != capture.AnswerSHA256 {
			t.Fatalf("native answer digest changed: %s", capture.ID)
		}
		files := picklistJSONNativeFiles(t, fixture, capture.Fixture)
		verifyClass := func(path string) {
			t.Helper()
			source, ok := files[path]
			if !ok || source.Source == "" || !strings.HasSuffix(path, ".cls") {
				t.Fatalf("missing class source: %s", path)
			}
			metadata, ok := files[path+"-meta.xml"]
			if !ok {
				t.Fatalf("missing source API metadata: %s", path)
			}
			var pin struct {
				APIVersion string `xml:"apiVersion"`
			}
			if err := xml.Unmarshal([]byte(metadata.Source), &pin); err != nil {
				t.Fatal(err)
			}
			if pin.APIVersion != capture.SourceAPI {
				t.Fatalf("source API mismatch: %s", path)
			}
		}
		verifyClass(capture.HelperPath)
		switch capture.Context {
		case "istest":
			if capture.TransportAPI != "67.0" {
				t.Fatal("named transport mismatch")
			}
			verifyClass(capture.ObserverPath)
		case "anonymous":
			if capture.TransportAPI != capture.SourceAPI || capture.AnonymousSource == "" || picklistJSONNativeDigest(capture.AnonymousSource) != capture.AnonymousSourceSHA256 {
				t.Fatal("anonymous source/transport mismatch")
			}
		default:
			t.Fatal("unobserved context")
		}
		answer := picklistJSONNativeAnswer(t, capture.AnswerJSON)
		fields := 0
		for field, raw := range answer {
			if !bytes.HasPrefix(bytes.TrimSpace(raw), []byte("[")) {
				continue
			}
			entries := picklistJSONNativeEntries(t, raw)
			fields++
			seenIndices := make(map[int]bool)
			for position, entry := range entries {
				index := position
				if entry.Index != nil {
					index = *entry.Index
				}
				if index < 0 || seenIndices[index] {
					t.Fatalf("invalid native entry index: %s", field)
				}
				seenIndices[index] = true
			}
		}
		if fields == 0 {
			t.Fatal("native answer has no observed entry arrays")
		}
	}
}

func picklistJSONApexLiteral(raw json.RawMessage) (string, error) {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return "null", nil
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		escaped := strings.NewReplacer("\\", "\\\\", "'", "\\'", "\r", "\\r", "\n", "\\n", "\t", "\\t").Replace(text)
		return "'" + escaped + "'", nil
	}
	var flag bool
	if err := json.Unmarshal(raw, &flag); err == nil {
		return fmt.Sprint(flag), nil
	}
	return "", fmt.Errorf("unobserved Apex fixture scalar: %s", raw)
}

// Assertions are derived from native rawJSON facets, not whole JSON key sets or
// property order. In particular, a present null validFor differs from an empty
// base64 string and the serializer's unrelated extra properties are unconstrained.
func TestPicklistJSONNativeConformance(t *testing.T) {
	fixture := loadPicklistJSONNativeFixture(t)
	for _, capture := range fixture.Captures {
		t.Run(capture.ID, func(t *testing.T) {
			root := t.TempDir()
			files := picklistJSONNativeFiles(t, fixture, capture.Fixture)
			p := project.Project{}
			for _, file := range files {
				path := filepath.Join(root, filepath.FromSlash(file.Path))
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(file.Source), 0644); err != nil {
					t.Fatal(err)
				}
				switch {
				case strings.HasSuffix(path, ".object-meta.xml"):
					p.ObjectFiles = append(p.ObjectFiles, path)
				case strings.HasSuffix(path, ".field-meta.xml"):
					p.FieldFiles = append(p.FieldFiles, path)
				}
			}
			sort.Strings(p.ObjectFiles)
			sort.Strings(p.FieldFiles)
			imported, err := schema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			if len(imported.Objects) != 1 {
				t.Fatalf("fixture has %d objects, want one explicit source object", len(imported.Objects))
			}
			objectName := imported.Objects[0].Name
			org := storage.NewOrgState()
			for name, describe := range sobject.BuildDescribeRegistry(imported).Objects {
				org.Objects[name] = storage.ObjectState{Definition: sobject.ToObjectDefinition(describe), Records: map[storage.ID]storage.Record{}}
			}
			answer := picklistJSONNativeAnswer(t, capture.AnswerJSON)
			var fields []string
			for field, raw := range answer {
				if bytes.HasPrefix(bytes.TrimSpace(raw), []byte("[")) {
					fields = append(fields, field)
				}
			}
			sort.Strings(fields)
			var body strings.Builder
			literal := func(raw json.RawMessage) string {
				t.Helper()
				text, err := picklistJSONApexLiteral(raw)
				if err != nil {
					t.Fatal(err)
				}
				return text
			}
			for _, field := range fields {
				fmt.Fprintf(&body, "{\nList<Schema.PicklistEntry> entries=Schema.getGlobalDescribe().get('%s').getDescribe().fields.getMap().get('%s').getDescribe().getPicklistValues();\n", objectName, field)
				if rawCount, ok := answer[field+"Count"]; ok {
					var count int
					if err := json.Unmarshal(rawCount, &count); err != nil {
						t.Fatal(err)
					}
					fmt.Fprintf(&body, "System.assertEquals(%d,entries.size());\n", count)
				}
				for position, entry := range picklistJSONNativeEntries(t, answer[field]) {
					index := position
					if entry.Index != nil {
						index = *entry.Index
					}
					var raw map[string]json.RawMessage
					if err := json.Unmarshal([]byte(entry.RawJSON), &raw); err != nil {
						t.Fatal(err)
					}
					fmt.Fprintf(&body, "{\nSchema.PicklistEntry entry=entries[%d];\nMap<String,Object> encoded=(Map<String,Object>)JSON.deserializeUntyped(JSON.serialize(entry));\nSystem.assertEquals(%t,encoded.containsKey('validFor'));\n", index, entry.HasValidFor)
					if value, ok := raw["validFor"]; ok {
						fmt.Fprintf(&body, "System.assertEquals(%s,encoded.get('validFor'));\n", literal(value))
						if !bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
							body.WriteString("System.assert(encoded.get('validFor') instanceof String);\n")
						}
					}
					for _, key := range []string{"value", "label", "active", "defaultValue"} {
						fmt.Fprintf(&body, "System.assertEquals(%s,encoded.get('%s'));\n", literal(raw[key]), key)
					}
					if entry.Hex != nil {
						encoded, _ := json.Marshal(*entry.Hex)
						fmt.Fprintf(&body, "System.assertEquals(%s,EncodingUtil.convertToHex(EncodingUtil.base64Decode((String)encoded.get('validFor'))));\n", literal(encoded))
					}
					body.WriteString("}\n")
				}
				body.WriteString("}\n")
			}
			// The separately observed ordinary map retains its public validFor
			// key. Assert its captured value without a whole-object JSON comparison.
			if unrelated, present := answer["unrelated"]; present {
				var raw string
				if err := json.Unmarshal(unrelated, &raw); err != nil {
					t.Fatal(err)
				}
				var control map[string]json.RawMessage
				if err := json.Unmarshal([]byte(raw), &control); err != nil {
					t.Fatal(err)
				}
				var keys []string
				for key := range control {
					keys = append(keys, key)
				}
				sort.Strings(keys)
				var pairs []string
				for _, key := range keys {
					encoded, _ := json.Marshal(key)
					pairs = append(pairs, literal(encoded)+"=>"+literal(control[key]))
				}
				fmt.Fprintf(&body, "{Map<String,Object> control=(Map<String,Object>)JSON.deserializeUntyped(JSON.serialize(new Map<String,Object>{%s}));\n", strings.Join(pairs, ","))
				for _, key := range keys {
					encoded, _ := json.Marshal(key)
					fmt.Fprintf(&body, "System.assertEquals(true,control.containsKey(%s));\nSystem.assertEquals(%s,control.get(%s));\n", literal(encoded), literal(control[key]), literal(encoded))
				}
				body.WriteString("}\n")
			}

			program, err := CompileAnonymousWithOptions(body.String(), CompileOptions{APIVersion: capture.SourceAPI})
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			machine.SetOrg(&org)
			switch capture.Context {
			case "istest":
				_, err = machine.ExecuteTestInClass(program, "PicklistJSONNativeReplay")
			case "anonymous":
				_, err = machine.Execute(program)
			default:
				t.Fatal("unobserved native context")
			}
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
