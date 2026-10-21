package apextest

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

const hierarchyNameOwnerFixtureDir = "testdata/conformance/hierarchy_name_owner_capture"

type hierarchyNameOwnerRow struct {
	ID, Priority, Owner, Name, Stage string
	LabelPresent                     bool
	ExpectedNative                   json.RawMessage
}

type hierarchyNameOwnerPacket struct {
	Status, ObjectName, Label string
	APIVersions, Routes       []string
	Rows                      []hierarchyNameOwnerRow
}

// Pending observers assert transport completeness, never the native values.
// Set GLADE_HIERARCHY_NAME_OWNER_CAPTURE_DIR to export independent deployable
// projects. Label-absent projects must be captured at the deployment boundary.
func TestHierarchyNameOwnerCaptureFixtures(t *testing.T) {
	data := hierarchyNameOwnerPacket{}
	read := func(name string) string {
		raw, err := os.ReadFile(filepath.Join(hierarchyNameOwnerFixtureDir, name))
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	if err := json.Unmarshal([]byte(read("packet.json")), &data); err != nil {
		t.Fatal(err)
	}
	if data.Status != "PENDING-ROWS" || len(data.Rows) != 9 || strings.Join(data.APIVersions, ",") != "62.0,67.0,68.0" || strings.Join(data.Routes, ",") != "anonymous,istest" {
		t.Fatal("incomplete capture matrix")
	}
	helper, metadata := read("HierarchyNameOwnerObserver.cls"), read("OwnerNamePreference__c.object-meta.xml")
	objects, tags := map[string]bool{}, map[string]bool{}
	for i, row := range data.Rows {
		if row.ID != fmt.Sprintf("HNO%02d", i+1) || string(row.ExpectedNative) != "null" || (row.Stage == "deployment") == row.LabelPresent {
			t.Fatal("capture row needs its isolated stage and an unknown native answer", row.ID)
		}
		label := ""
		if row.LabelPresent {
			label = "  <label>" + data.Label + "</label>"
		}
		object := strings.ReplaceAll(metadata, "{{LABEL}}", label)
		for _, api := range data.APIVersions {
			for _, route := range data.Routes {
				suffix := "A"
				if route == "istest" {
					suffix = "T"
				}
				objectName := "NameOwner" + strings.TrimSuffix(api, ".0") + suffix + row.ID + "__c"
				tag := row.ID + "-" + api + "-" + suffix
				if objects[objectName] || tags[tag] {
					t.Fatal("capture cells share a hierarchy object or transport tag")
				}
				objects[objectName], tags[tag] = true, true
				t.Run(row.ID+"/"+api+"/"+route, func(t *testing.T) {
					name, classes, seed, parts := hierarchyNameOwnerSources(row, route, strings.ReplaceAll(helper, data.ObjectName, objectName), tag)
					if export := os.Getenv("GLADE_HIERARCHY_NAME_OWNER_CAPTURE_DIR"); export != "" {
						hierarchyNameOwnerExport(t, filepath.Join(export, row.ID, api, route), api, route, row, objectName, object, tag, classes, seed, parts)
					}
					if row.Stage == "deployment" {
						if strings.Contains(object, "<label>") {
							t.Fatal("label-absent deployment control acquired a label")
						}
						return // Native deployment outcome remains uncaptured.
					}
					index := nativeBoundaryIndex(t, api, classes, map[string]string{objectName: object})
					if index.HasErrors() || sema.Analyze(index).HasErrors() {
						t.Fatal("capture fixture does not compile", index.Diagnostics, sema.Analyze(index).Diagnostics)
					}
					var messages []string
					if route == "istest" {
						run := Run(index, Options{SelectedClasses: []string{name}, NoDiskCache: true, Parallelism: 1})
						for _, suite := range run.Suites {
							for _, c := range suite.Cases {
								if c.Problem == nil || c.Problem.Type != "System.AssertException" {
									t.Fatalf("missing intentional chunk assertion: %+v", c)
								}
								messages = append(messages, c.Problem.Message)
							}
						}
					} else {
						org := orgFromIndex(index)
						org.APIVersion = api
						machine := newConformanceRunner(t, index, conformanceRunnerOptions{Org: &org, LinkProject: true}).newMachine()
						execute := func(source string) error {
							p, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
							if err != nil {
								t.Fatal(err)
							}
							_, err = machine.Execute(p)
							return err
						}
						if err := execute(seed); err != nil {
							t.Fatal("capture seed failed", err)
						}
						for _, source := range parts {
							var runtimeErr *vm.RuntimeError
							if err := execute(source); !errors.As(err, &runtimeErr) || runtimeErr.Type != "System.AssertException" {
								t.Fatal("missing intentional chunk assertion", err)
							} else {
								messages = append(messages, runtimeErr.ExceptionMessage())
							}
						}
					}
					hierarchyNameOwnerCheckChunks(t, tag, messages)
				})
			}
		}
	}
}

func hierarchyNameOwnerSources(row hierarchyNameOwnerRow, route, helper, tag string) (string, map[string]string, string, []string) {
	name := "HierarchyNameOwner" + row.ID
	seed := fmt.Sprintf("HierarchyNameOwnerObserver.seed('%s','%s','%s');", tag, row.Owner, row.Name)
	classes := map[string]string{"HierarchyNameOwnerObserver": helper}
	var parts []string
	var named strings.Builder
	fmt.Fprintf(&named, "@IsTest private class %s { @TestSetup static void seed() {%s}\n", name, seed)
	for part := 0; part < 32; part++ {
		source := fmt.Sprintf("System.assert(false,HierarchyNameOwnerObserver.part('%s',%d));", tag, part)
		parts = append(parts, source)
		fmt.Fprintf(&named, "@IsTest static void part%02d() {%s}\n", part, source)
	}
	named.WriteString("}\n")
	if route == "istest" {
		classes[name] = named.String()
	}
	return name, classes, seed, parts
}

func hierarchyNameOwnerCheckChunks(t *testing.T, tag string, messages []string) {
	t.Helper()
	chunks := make([]string, 32)
	seen := map[int]bool{}
	hash := ""
	for _, message := range messages {
		_, encoded, found := strings.Cut(message, "HNAME_RESULT|"+tag+"|")
		var part struct {
			Part, Total int
			Hash, Chunk string
		}
		if !found || json.Unmarshal([]byte(encoded), &part) != nil || part.Total != 32 || part.Part < 0 || part.Part >= 32 || seen[part.Part] || len(part.Chunk) > 320 || (hash != "" && hash != part.Hash) {
			t.Fatal("invalid or mixed capture transport", message)
		}
		seen[part.Part], hash, chunks[part.Part] = true, part.Hash, part.Chunk
	}
	raw, err := base64.StdEncoding.DecodeString(strings.Join(chunks, ""))
	sum := sha256.Sum256(raw)
	if err != nil || len(seen) != 32 || hex.EncodeToString(sum[:]) != hash {
		t.Fatal("incomplete capture payload")
	}
	var payload map[string]any
	if json.Unmarshal(raw, &payload) != nil || payload["stage"] != "complete" {
		t.Fatal("measurement did not finish", string(raw))
	}
	for _, view := range []string{"input", "callerAfterDml", "soql", "orgDefaults", "instance", "instanceOrganization", "instanceUser", "instanceProfile"} {
		observation, ok := payload[view].(map[string]any)
		if !ok {
			if _, caught := payload["measurementError"]; caught && (view == "input" || view == "callerAfterDml") {
				continue
			}
			t.Fatal("missing measured view", view, string(raw))
		}
		if _, caught := observation["exceptionType"]; caught {
			continue // Capture rejection as data; do not infer a native value.
		}
		for _, field := range []string{"Name", "SetupOwnerId", "ownerIsOrganization", "ownerIsUser", "ownerIsProfile", "nameEqualsOwnerString", "nameEqualsOrganizationString", "nameEqualsUserString", "nameEqualsProfileString"} {
			if _, measured := observation[field]; !measured {
				t.Fatal("missing independent Name/owner observation", view, field)
			}
		}
	}
}

func hierarchyNameOwnerExport(t *testing.T, root, api, route string, row hierarchyNameOwnerRow, objectName, object, tag string, classes map[string]string, seed string, parts []string) {
	t.Helper()
	manifest, err := json.MarshalIndent(map[string]any{"row": row.ID, "api": api, "route": route, "priority": row.Priority, "stage": row.Stage, "object": objectName, "transportTag": tag, "owner": row.Owner, "name": row.Name, "labelPresent": row.LabelPresent, "expectedNative": nil}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "capture.json"), string(manifest)+"\n")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
	for name, source := range classes {
		path := filepath.Join(root, "force-app/main/default/classes", name+".cls")
		writeFile(t, path, source)
		writeFile(t, path+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
	}
	writeFile(t, filepath.Join(root, "force-app/main/default/objects", objectName, objectName+".object-meta.xml"), object)
	writeFile(t, filepath.Join(root, "seed.apex"), seed)
	for i, source := range parts {
		writeFile(t, filepath.Join(root, fmt.Sprintf("part-%02d.apex", i)), source)
	}
}
