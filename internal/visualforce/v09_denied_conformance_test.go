package visualforce_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"golang.org/x/net/html"
)

type v09DeniedTable struct {
	OracleCommit      string `json:"oracle_commit"`
	SourceFingerprint string `json:"source_fingerprint"`
	Fixtures          map[string]struct {
		Files       map[string]string `json:"files"`
		ContextPage string            `json:"context_page"`
	} `json:"fixtures"`
	Cases []struct {
		ID       string                     `json:"id"`
		Kind     string                     `json:"kind"`
		Phase    string                     `json:"phase"`
		Inputs   map[string]v04Input        `json:"inputs"`
		Expected map[string]string          `json:"expected"`
		Access   map[string]map[string]bool `json:"access"`
	} `json:"cases"`
}

// Each native phase used the same restricted principal, supplied values and
// page/controller sources. Recreate only the captured object/field grants;
// native DOM answers never configure execution. Verify the controller's native
// describe/identity guard locally before comparing each exact DOM row.
func v09DeniedOrg(index typesys.Index, access map[string]bool) storage.OrgState {
	org, principal := v09Org(index)
	org.OrgID = "00D000000000mH2EAI"
	user := org.Objects["User"].Records[principal]
	profileID := user.Fields["ProfileId"].ID
	profiles := org.Objects["Profile"]
	profile := profiles.Records[profileID]
	profile.Fields["Name"] = storage.StringValue("FamilyV09DeniedRestricted")
	profile.Fields["PermissionsViewAllData"] = storage.BooleanValue(false)
	profile.Fields["PermissionsModifyAllData"] = storage.BooleanValue(false)
	profiles.Records[profileID] = profile
	org.Objects["Profile"] = profiles
	objects := org.Objects["ObjectPermissions"]
	for id, permission := range objects.Records {
		if permission.Fields["SObjectType"].String != "V09FieldFixture__c" {
			continue
		}
		permission.Fields["PermissionsRead"] = storage.BooleanValue(access["objectRead"])
		permission.Fields["PermissionsCreate"] = storage.BooleanValue(access["objectCreate"])
		permission.Fields["PermissionsEdit"] = storage.BooleanValue(access["objectUpdate"])
		permission.Fields["PermissionsDelete"] = storage.BooleanValue(false)
		objects.Records[id] = permission
	}
	org.Objects["ObjectPermissions"] = objects
	fields := org.Objects["FieldPermissions"]
	for id, permission := range fields.Records {
		var prefix string
		switch permission.Fields["Field"].String {
		case "V09FieldFixture__c.Text__c":
			prefix = "text"
		case "V09FieldFixture__c.Lookup__c":
			prefix = "lookup"
		default:
			continue
		}
		permission.Fields["PermissionsRead"] = storage.BooleanValue(access[prefix+"Read"])
		permission.Fields["PermissionsEdit"] = storage.BooleanValue(access[prefix+"Update"])
		fields.Records[id] = permission
	}
	org.Objects["FieldPermissions"] = fields
	return org
}

func v09ObserveDeniedControl(t *testing.T, doc *html.Node, request *http.Request, status int) string {
	t.Helper()
	if guard := v04Find(doc, func(n *html.Node) bool { return v04Attr(n, "data-v09-access-context") == "true" }); guard != nil {
		var access map[string]bool
		if err := json.Unmarshal([]byte(v09Text(guard)), &access); err != nil {
			return "ACCESS|invalid native context: " + err.Error()
		}
		return "ACCESS|" + v04Encode(t, access)
	}
	marker := v04Find(doc, func(n *html.Node) bool { return v04Attr(n, "data-v09-denied-row") == "true" })
	if marker == nil {
		return v09DOM(t, map[string]any{"localError": "denied control marker missing", "status": status})
	}
	stable := func(text string) string {
		for _, role := range [][2]string{{v09AccountID, "OWNED_ACCOUNT_ID"}, {v09AccountID[:15], "OWNED_ACCOUNT_ID"}, {v09AccountName, "OWNED_ACCOUNT_NAME"}} {
			text = strings.ReplaceAll(text, role[0], role[1])
		}
		return text
	}
	idmap, nextID := map[string]string{}, 0
	v04Walk(marker, func(n *html.Node) {
		if id := v04Attr(n, "id"); id != "" {
			idmap[id] = fmt.Sprintf("owned-id-%d", nextID)
			nextID++
		}
	})
	ownedInputs := map[string]bool{}
	v04Walk(marker, func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "input" && v09ControlType(n) != "hidden" {
			ownedInputs[v04Attr(n, "id")] = true
		}
	})
	controls, hidden, links, labels, tags := []any{}, []any{}, []any{}, []any{}, []any{}
	v04Walk(marker, func(n *html.Node) {
		if n.Type != html.ElementNode || n == marker {
			return
		}
		switch n.Data {
		case "input", "select", "textarea":
			kind, id := v09ControlType(n), v04Attr(n, "id")
			if kind == "hidden" {
				for _, suffix := range []string{"_lkid", "_lkold", "_lktp", "_lspf", "_lspfsub", "_mod", "mod"} {
					field := strings.TrimSuffix(id, suffix)
					if field != id && ownedInputs[field] {
						hidden = append(hidden, map[string]any{"field": idmap[field], "suffix": suffix, "value": stable(v04Attr(n, "value"))})
						break
					}
				}
				return
			}
			value, _ := v09ControlValue(n, kind)
			var tabindex any
			if v04HasAttr(n, "tabindex") {
				tabindex = v04Attr(n, "tabindex")
			}
			controls = append(controls, map[string]any{"tag": n.Data, "type": kind, "value": stable(value),
				"disabled": v04HasAttr(n, "disabled"), "required": v04HasAttr(n, "required"), "id": idmap[id],
				"class": v04Attr(n, "class"), "style": v04Attr(n, "style"), "tabindex": tabindex})
		case "label":
			labels = append(labels, map[string]any{"text": stable(v09Text(n)), "for": idmap[v04Attr(n, "for")]})
		case "a":
			if v04HasAttr(n, "href") {
				href := v04Attr(n, "href")
				if parsed, err := url.Parse(href); err == nil {
					target := request.URL.ResolveReference(parsed)
					if target.Scheme == request.URL.Scheme && target.Host == request.URL.Host {
						href = target.EscapedPath()
						if href == "" {
							href = "/"
						}
					}
				}
				links = append(links, map[string]any{"text": stable(v09Text(n)), "href": stable(href)})
			}
		}
	})
	var text strings.Builder
	v09WalkClone(marker, func(n *html.Node) {
		if n.Type == html.TextNode {
			text.WriteString(n.Data)
		} else if n.Type == html.ElementNode && n != marker {
			tags = append(tags, n.Data)
		}
	})
	return v09DOM(t, map[string]any{"text": stable(text.String()), "controls": controls, "hidden": hidden, "links": links, "labels": labels, "tags": tags})
}

func v09RunDeniedControls(t *testing.T, capture bool, writeRow func(...string)) {
	t.Helper()
	data, err := os.ReadFile("testdata/v09_denied_controls.json")
	if err != nil {
		t.Fatal(err)
	}
	var table v09DeniedTable
	if err := json.Unmarshal(data, &table); err != nil {
		t.Fatal(err)
	}
	if table.OracleCommit != "capture-revision-048137615117366033878447963286093391879574679956782728413088579616007998642802" ||
		table.SourceFingerprint != "6f66cefd6aa795d5cc0d53724eff1d35b85afc40e38d5ad8a8c41f1d1c0f3920" || len(table.Cases) != 20 {
		t.Fatal("incomplete captured V09 denied controls")
	}
	total := 0
	for _, api := range []string{"59.0", "67.0"} {
		root := t.TempDir()
		fixture := table.Fixtures[api]
		if len(fixture.Files) == 0 || fixture.ContextPage == "" {
			t.Fatal("missing V09 denied source scaffold at API " + api)
		}
		v09WriteFiles(t, root, fixture.Files)
		renderer := v09NewRenderer(t, root)
		matches, compileRows, runtimeRows := 0, 0, 0
		seen := map[string]bool{}
		for _, row := range table.Cases {
			if row.ID == "" || seen[row.ID] || (row.Kind != "compile" && row.Kind != "runtime") || row.Expected[api] == "" {
				t.Fatal("invalid or duplicate native denied row " + row.ID)
			}
			seen[row.ID] = true
			func() {
				input := row.Inputs[api]
				if input.Name == "" || len(input.Files) != 2 {
					t.Fatal("incomplete native denied input " + row.ID)
				}
				v09WriteFiles(t, root, input.Files)
				defer func() {
					for path := range input.Files {
						if err := os.Remove(filepath.Join(root, path)); err != nil {
							t.Error(err)
						}
					}
				}()
				got, actualAccess, expectedAccess := "", "", ""
				if row.Kind == "compile" {
					compileRows++
					var diagnostics []v04Diagnostic
					got, diagnostics = v04Execute(t, root, input.Name, v04Case{Kind: "compile"})
					actualAccess, expectedAccess = v04Encode(t, diagnostics), "[]"
				} else {
					runtimeRows++
					access := row.Access[api]
					accessKeys := []string{"actorMatches", "orgMatches", "accountRead", "objectRead", "objectCreate", "objectUpdate", "textRead", "textCreate", "textUpdate", "lookupRead", "lookupCreate", "lookupUpdate"}
					if len(access) != len(accessKeys) || !access["actorMatches"] || !access["orgMatches"] || !access["accountRead"] {
						t.Fatal("missing native denied principal/access facts " + row.ID)
					}
					for _, key := range accessKeys {
						if _, present := access[key]; !present {
							t.Fatalf("missing native denied access fact %s in %s", key, row.ID)
						}
					}
					renderer.org = storage.NewRuntimeTemplate(v09DeniedOrg(renderer.index, access))
					c := v09Case{ID: row.ID, DeniedControl: true, Parameters: map[string]string{
						"principal": string(renderer.principal), "captureOrg": "00D000000000mH2EAI", "lookupId": v09AccountID,
					}}
					actualAccess = renderer.render(t, fixture.ContextPage, c)
					expectedAccess = "ACCESS|" + v04Encode(t, access)
					got = renderer.render(t, input.Name, c)
				}
				status := "MISMATCH"
				if got == row.Expected[api] && actualAccess == expectedAccess {
					status = "MATCH"
					matches++
				} else if !capture {
					t.Errorf("V09 denied API %s %s actual <%s> expected <%s>; access/diagnostics <%s> expected <%s>",
						api, row.ID, got, row.Expected[api], actualAccess, expectedAccess)
				}
				writeRow(api, row.ID, status, got, row.Expected[api], "denied-controls", actualAccess, "", "", "", "", expectedAccess)
			}()
			v09ReleaseRowState(t, root)
		}
		if compileRows != 5 || runtimeRows != 15 {
			t.Fatalf("incomplete V09 denied row coverage: compile %d, runtime %d", compileRows, runtimeRows)
		}
		renderer = nil
		v09ReleaseRowState(t, root)
		total += matches
		t.Logf("V09 denied API %s exact %d/20", api, matches)
	}
	writeRow("DENIED_TOTAL", fmt.Sprintf("%d/40", total))
	t.Logf("V09 denied controls exact %d/40", total)
}
