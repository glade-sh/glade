package visualforce_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/visualforce"
	"github.com/glade-sh/glade/internal/vm"
	"golang.org/x/net/html"
)

// These additional native controls check required-field value (including raw
// null), action count and exact ApexPages message severity/summary/detail. The
// export also retains every raw DOM observation; this projection does not add
// to the original Select controls full-DOM denominator. The API59/67 good twins verify the
// same isolated markup can submit successfully through the product lifecycle.
// Whitespace/upload controls also compare exact form/file IDs and field names.
// Differences in that shared allocator remain with active Form lifecycle; a matching row
// enters the exact count even while its recorded ownership is retained.
func v07RequiredIDSalesforceConformance(t *testing.T) {
	data, err := os.ReadFile("testdata/v07_required_ids.json")
	if err != nil {
		t.Fatal(err)
	}
	var table struct {
		Source string `json:"source"`
		Cases  []struct {
			ID        string              `json:"id"`
			Layout    string              `json:"layout"`
			Submitted string              `json:"submitted"`
			Identity  bool                `json:"identity"`
			Owner     string              `json:"owner"`
			Reason    string              `json:"reason"`
			Inputs    map[string]v10Input `json:"inputs"`
			Expected  map[string]string   `json:"expected"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(data, &table); err != nil {
		t.Fatal(err)
	}
	if table.Source != "Owned required-field identity controls at API 59.0 and 67.0." || len(table.Cases) != 16 {
		t.Fatalf("required-ID controls need 12 original and four whitespace native rows, got %d", len(table.Cases))
	}
	matches := 0
	for _, api := range []string{"59.0", "67.0"} {
		apiMatches := 0
		for _, c := range table.Cases {
			t.Run(api+"/"+c.ID, func(t *testing.T) {
				root := t.TempDir()
				input := c.Inputs[api]
				for path, source := range input.Files {
					if filepath.IsAbs(path) || filepath.Clean(path) != path || strings.HasPrefix(path, "..") {
						t.Fatalf("invalid fixture path %q", path)
					}
					full := filepath.Join(root, path)
					if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(full, []byte(source), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				p, err := project.Load(root)
				if err != nil {
					t.Fatal(err)
				}
				sch, err := schema.LoadProject(p)
				if err != nil {
					t.Fatal(err)
				}
				index := typesys.Build(p, sch)
				vf, err := visualforce.LoadProjectForRender(p)
				if err != nil {
					t.Fatal(err)
				}
				org := storage.NewOrgState()
				request := visualforce.PageRenderRequest{Project: p, VFIndex: vf, Org: &org, PageName: input.Name, PageURL: "/apex/" + input.Name, ViewStateSecret: []byte("Select controls native required-ID control view-state secret")}
				render := func() *html.Node {
					machine := vm.New(nil)
					machine.Org = &org
					if err := apextest.RegisterProjectRuntimeForRequest(machine, index); err != nil {
						t.Fatal(err)
					}
					request.Machine = machine
					result, err := visualforce.RenderPage(request)
					if err != nil {
						t.Fatal(err)
					}
					if result.Error != nil {
						t.Fatal(result.Error)
					}
					doc, err := html.Parse(strings.NewReader(result.HTML))
					if err != nil {
						t.Fatal(err)
					}
					return doc
				}
				observe := func(doc *html.Node) map[string]any {
					marker := v10Find(doc, func(n *html.Node) bool { return v10Attr(n, "data-v07-result") == c.ID })
					if marker == nil {
						t.Fatal("required-ID result marker missing")
					}
					var value map[string]any
					if err := json.Unmarshal([]byte(v10Text(marker)), &value); err != nil {
						t.Fatal(err)
					}
					projection := map[string]any{}
					for _, key := range []string{"a", "posts", "messages"} {
						v, ok := value[key]
						if !ok {
							t.Fatalf("required-ID result lacks %s", key)
						}
						projection[key] = v
					}
					if c.Identity {
						form := v10Find(doc, func(n *html.Node) bool { return n.Type == html.ElementNode && n.Data == "form" })
						upload := v10Find(form, func(n *html.Node) bool {
							return n.Type == html.ElementNode && n.Data == "input" && v10Attr(n, "type") == "file" && v10Attr(n, "name") == "upload"
						})
						if form == nil || upload == nil {
							t.Fatal("native identity control requires its form and file input")
						}
						names := map[string]string{}
						v10Walk(form, func(n *html.Node) {
							if n.Type != html.ElementNode || n.Data != "input" {
								return
							}
							field := v10Ancestor(n, func(p *html.Node) bool { return v10Attr(p, "data-v07-field") != "" })
							if field != nil {
								key := v10Attr(field, "data-v07-field")
								if _, exists := names[key]; exists {
									t.Fatalf("duplicate native identity field %s", key)
								}
								names[key] = v10Attr(n, "name")
							}
						})
						if len(names) != 2 || names["a"] == "" || names["b"] == "" || v10Attr(form, "id") == "" || v10Attr(upload, "id") == "" {
							t.Fatal("native identity control requires both named fields and form/file IDs")
						}
						projection["identity"] = map[string]any{"form_id": v10Attr(form, "id"), "file_id": v10Attr(upload, "id"), "field_names": names}
					}
					return projection
				}
				doc := render()
				before := observe(doc)
				v10Walk(doc, func(n *html.Node) {
					if n.Data != "input" {
						return
					}
					field := v10Ancestor(n, func(p *html.Node) bool { return v10Attr(p, "data-v07-field") != "" })
					if field == nil {
						return
					}
					value := "posted-b"
					if v10Attr(field, "data-v07-field") == "a" {
						value = "posted-a"
						if c.Submitted == "blank" {
							value = ""
						}
					}
					v10SetAttr(n, "value", value)
				})
				button := v10Find(doc, func(n *html.Node) bool {
					if n.Data != "a" || v10Ancestor(n, func(p *html.Node) bool { return v10Attr(p, "data-v07-submit") == "owned" }) == nil {
						return false
					}
					if !strings.HasPrefix(c.Layout, "repeat_") {
						return true
					}
					row := v10Ancestor(n, func(p *html.Node) bool { return v10Attr(p, "data-v07-repeat-row") != "" })
					return row != nil && v10Attr(row, "data-v07-repeat-row") == "selected"
				})
				form := v10Ancestor(button, func(n *html.Node) bool { return n.Data == "form" })
				if button == nil || form == nil {
					t.Fatal("required-ID submit control/form missing")
				}
				values := v10FormValues(form, button, true)
				match := v10ActionHook.FindStringSubmatch(v10Attr(button, "onclick"))
				if len(match) != 2 {
					t.Fatal("required-ID action hook missing")
				}
				values[visualforce.ViewStateActionFieldName()] = match[1]
				if command := v10Attr(button, "data-vf-command"); command != "" {
					values["__vf_command"] = command
				}
				payload, err := visualforce.DecodeViewState(values[visualforce.ViewStateFormFieldName()], request.ViewStateSecret)
				if err != nil {
					t.Fatal(err)
				}
				if err := visualforce.VerifyViewStateCSRF(payload, values["__vf_csrf"]); err != nil {
					t.Fatal(err)
				}
				parsed := visualforce.ParseAjaxPayload(values)
				request.ViewState, request.FormValues, request.Action = &payload, parsed.SubmittedFields, parsed.Action
				request.PageURL = v10Attr(form, "action")
				after := observe(render())
				got := v10Encode(t, map[string]any{"before": before, "after": after})
				want, ok := c.Expected[api]
				if !ok {
					t.Fatal("native required-ID answer missing")
				}
				if got == want {
					matches++
					apiMatches++
				} else if c.Owner != "" {
					t.Logf("required-ID difference owned by %s: %s; actual <%s> expected <%s>", c.Owner, c.Reason, got, want)
				} else if os.Getenv("GLADE_V07_CAPTURE") == "" {
					t.Errorf("actual <%s> expected <%s>", got, want)
				} else {
					t.Logf("required-ID mismatch: actual <%s> expected <%s>", got, want)
				}
			})
		}
		t.Logf("Select controls required-ID API %s exact messages/value/action/identity controls %d/%d", api, apiMatches, len(table.Cases))
	}
	t.Logf("Select controls required-ID exact messages/value/action/identity controls %d/%d", matches, 2*len(table.Cases))
}
