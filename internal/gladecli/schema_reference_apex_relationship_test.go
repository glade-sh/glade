package gladecli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/lsp"
	"github.com/glade-sh/glade/internal/testreport"
)

// End-to-end twins retain a project error for unused invalid metadata and
// exclude ordinary Apex relationship users and their transitive dependents.
func TestSchemaReferenceGateApexRelationshipCLIProjectRoutes(t *testing.T) {
	t.Setenv("GLADE_HOME", t.TempDir())
	t.Setenv("GLADE_ROOT", "")
	var data struct {
		Cases []struct {
			API, ID, Name, Source, Target string
			References, NativeValid       bool
			ExtraFiles                    map[string]string
		}
		Metadata map[string]map[string]string
	}
	raw, err := os.ReadFile("../sema/testdata/schema_relationship_gate/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	selected := map[string]bool{"apex_parent": true, "apex_custom": true, "apex_owner": true, "apex_shadow": true, "unused_metadata": true, "apex_inherited": true, "apex_overload": true}
	for _, c := range data.Cases {
		if !selected[c.ID] {
			continue
		}
		t.Run(c.API+"/"+c.ID, func(t *testing.T) {
			root := t.TempDir()
			writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"src","default":true}],"sourceApiVersion":"`+c.API+`"}`)
			for path, source := range data.Metadata[c.API] {
				writeTestFile(t, filepath.Join(root, "src", path), source)
			}
			for path, source := range c.ExtraFiles {
				writeTestFile(t, filepath.Join(root, "src", path), source)
			}
			field := "Destination" + strings.TrimSuffix(c.API, ".0") + "__c"
			fieldPath := filepath.Join("src/objects", c.Target, "fields", field+".field-meta.xml")
			writeTestFile(t, filepath.Join(root, fieldPath), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>`+field+`</fullName><type>Lookup</type><referenceTo>UnavailableDestination</referenceTo><relationshipName>Destinations</relationshipName></CustomField>`)
			writeTestFile(t, filepath.Join(root, "src/classes", c.Name+".cls"), c.Source)
			writeTestFile(t, filepath.Join(root, "src/classes/RelationshipTest.cls"), `@IsTest private class RelationshipTest {@IsTest static void observe(){System.assert(`+c.Name+`.value());}}`)
			writeTestFile(t, filepath.Join(root, "src/classes/IndependentTest.cls"), `@IsTest private class IndependentTest {@IsTest static void observe(){System.assertEquals(7,7);}}`)
			for _, cached := range []bool{false, true, true} {
				var out, errs bytes.Buffer
				args := []string{"test", "--project", root, "--no-serve", "--no-progress", "--json"}
				if !cached {
					args = append(args, "--no-cache")
				}
				code := Run(context.Background(), args, &out, &errs)
				run, err := decodeTestRunJSON(out.Bytes())
				if err != nil {
					t.Fatalf("decode: %v out=%s stderr=%s", err, out.String(), errs.String())
				}
				got := map[string]testreport.Status{}
				for _, s := range run.Suites {
					for _, tc := range s.Cases {
						got[tc.ClassName] = tc.Status
					}
				}
				want := testreport.StatusPass
				if c.References {
					want = testreport.StatusCompileError
				}
				if code != 1 || run.Summary().OK() || got["IndependentTest"] != testreport.StatusPass || got["RelationshipTest"] != want || (!c.References && got["project"] != testreport.StatusCompileError) {
					t.Fatalf("cached=%t code=%d statuses=%v exclusions=%+v stderr=%s", cached, code, got, run.ExcludedClasses, errs.String())
				}
			}
			t.Run("check", func(t *testing.T) {
				var out, errs bytes.Buffer
				if code := Run(context.Background(), []string{"check", "--project", root, "--no-progress", "--json"}, &out, &errs); code != 1 || !strings.Contains(out.String(), "GLADESEMA003") {
					t.Fatalf("check code=%d out=%s stderr=%s", code, out.String(), errs.String())
				}
			})
			t.Run("lsp_metadata", func(t *testing.T) {
				index, err := loadIndex(root)
				if err != nil {
					t.Fatal(err)
				}
				h := lsp.NewHandler(index)
				defer h.HandleJSON([]byte(`{"jsonrpc":"2.0","id":1,"method":"shutdown"}`))
				uri := lsp.DocumentURI((&url.URL{Scheme: "file", Path: filepath.Join(root, fieldPath)}).String())
				notes := h.DidClose(lsp.DidCloseTextDocumentParams{TextDocument: lsp.TextDocumentIdentifier{URI: uri}})
				encoded, _ := json.Marshal(notes)
				if !strings.Contains(string(encoded), "GLADESEMA003") {
					t.Fatalf("metadata error missing from LSP: %s", encoded)
				}
			})
			for _, route := range []struct{ name, source string }{{"anonymous", "System.assertEquals(7,7);"}, {"named", "IndependentTest.observe();"}} {
				// The named call uses a public helper; @IsTest execution is above.
				writeTestFile(t, filepath.Join(root, "src/classes/Independent.cls"), `public class Independent {public static Integer value(){return 7;}}`)
				if route.name == "named" {
					route.source = "System.assertEquals(7,Independent.value());"
				}
				t.Run(route.name, func(t *testing.T) {
					var out, errs bytes.Buffer
					if code := Run(context.Background(), []string{"exec", "--project", root, "--json", route.source}, &out, &errs); code != 0 {
						t.Fatalf("exec code=%d out=%s stderr=%s", code, out.String(), errs.String())
					}
				})
			}
		})
	}
}
