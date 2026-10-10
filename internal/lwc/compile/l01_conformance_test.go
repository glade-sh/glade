package compile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/lwc"
	"golang.org/x/net/html"
)

type l01Case struct {
	ID              string            `json:"id"`
	Kind            string            `json:"kind"`
	Template        string            `json:"template"`
	JS              string            `json:"js"`
	MetaFragment    string            `json:"meta_fragment"`
	OmitVersion     bool              `json:"omit_version"`
	Expected        map[string]string `json:"expected"`
	RuntimeExpected map[string]string `json:"runtime_expected"`
}

// TestL01SalesforceConformance uses only exported, owned Salesforce observations.
// Capture mode reports actual results on the accepted tree before product edits.
func TestL01SalesforceConformance(t *testing.T) {
	capture := os.Getenv("GLADE_L01_CAPTURE") != ""
	table := "testdata/l01_salesforce.json"
	versions := []string{"59.0", "67.0"}
	kind := ""
	if capture {
		if override := os.Getenv("GLADE_L01_TABLE"); override != "" {
			table = override
		}
		if override := strings.Fields(os.Getenv("GLADE_L01_VERSIONS")); len(override) != 0 {
			versions = override
		}
		kind = os.Getenv("GLADE_L01_KIND")
	}
	data, err := os.ReadFile(table)
	if err != nil {
		t.Fatal(err)
	}
	var cases []l01Case
	if err = json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) < 150 {
		t.Fatalf("short L01 case table: %d", len(cases))
	}
	if !capture {
		if len(cases) != 180 {
			t.Fatalf("L01 conformance requires all 180 native cases, got %d", len(cases))
		}
		observedVersions := map[string]bool{}
		for _, c := range cases {
			for _, api := range versions {
				if c.Expected[api] == "" {
					t.Fatalf("missing native compiler answer for %s at API %s", c.ID, api)
				}
			}
			for api := range c.Expected {
				observedVersions[api] = true
			}
		}
		versions = nil
		for api := range observedVersions {
			versions = append(versions, api)
		}
		sort.Strings(versions)
	}
	var report strings.Builder
	matches, total := 0, 0
	for _, api := range versions {
		compiled := compileConformanceBatch(t, api, len(cases), func(root string, index int) {
			l01WriteBundle(t, root, cases[index], api)
		})
		for index, c := range cases {
			// Intermediate APIs enforce only rows with native bisect answers.
			if !capture {
				if _, observed := c.Expected[api]; !observed {
					continue
				}
			}
			if kind != "" && c.Kind != kind {
				continue
			}
			compileErr := compiled[index].Err
			got := "COMPILE_OK"
			if compileErr != nil {
				got = "COMPILE_ERROR"
			}
			want := c.Expected[api]
			runtimeWant, hasRuntimeOracle := c.RuntimeExpected[api]
			if c.Kind == "runtime" {
				want = "" // Native DOM capture is required; compile success alone is insufficient.
				if hasRuntimeOracle {
					want = "TEXT|" + runtimeWant // A captured empty string is a valid DOM answer.
				}
			}
			if c.Kind == "runtime" && compileErr == nil && hasRuntimeOracle {
				tree, parseErr := lwc.ParseTemplate(c.Template)
				if parseErr != nil {
					t.Fatal(parseErr)
				}
				markup, renderErr := lwc.RenderTemplate(tree, &lwc.RenderContext{Properties: lwc.PropertyBag{"value": lwc.StringValue("VALUE"), "flag": lwc.BoolValue(true), "row": lwc.ObjectValue(map[string]lwc.Value{"value": lwc.StringValue("NESTED")})}})
				if renderErr != nil {
					got = "RENDER_ERROR"
				} else {
					doc, parseErr := html.Parse(strings.NewReader(markup))
					if parseErr != nil {
						t.Fatal(parseErr)
					}
					var text strings.Builder
					var visit func(*html.Node)
					visit = func(n *html.Node) {
						if n.Type == html.TextNode {
							text.WriteString(n.Data)
						}
						for child := n.FirstChild; child != nil; child = child.NextSibling {
							visit(child)
						}
					}
					visit(doc)
					got = "TEXT|" + text.String()
				}
			}
			total++
			status := "UNKNOWN"
			if want != "" {
				status = "MISMATCH"
				if got == want {
					matches++
					status = "MATCH"
				}
			}
			fmt.Fprintf(&report, "%s\t%s\t%s\t%s\t%s\n", api, c.ID, status, got, want)
			if !capture && (want == "" || got != want) {
				t.Errorf("%s %s got %s want %s (compile error %v)", api, c.ID, got, want, compileErr)
			}
		}
	}
	fmt.Fprintf(&report, "TOTAL\t%d/%d\n", matches, total)
	t.Logf("L01 matches %d/%d", matches, total)
	if path := os.Getenv("GLADE_L01_REPORT"); path != "" {
		if err = os.WriteFile(path, []byte(report.String()), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func l01WriteBundle(t *testing.T, root string, c l01Case, api string) {
	t.Helper()
	bundle := filepath.Join(root, "force-app", "main", "default", "lwc", "familyCompile")
	writeCompileFixtureFile(t, filepath.Join(bundle, "familyCompile.js"), c.JS)
	writeCompileFixtureFile(t, filepath.Join(bundle, "familyCompile.html"), c.Template)
	version := "<apiVersion>" + api + "</apiVersion>"
	if c.OmitVersion {
		version = ""
	}
	fragment := c.MetaFragment
	if fragment == "" {
		fragment = "<isExposed>false</isExposed>"
	}
	meta := `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata">` + version + strings.ReplaceAll(fragment, "{api}", api) + `</LightningComponentBundle>`
	writeCompileFixtureFile(t, filepath.Join(bundle, "familyCompile.js-meta.xml"), meta)
}
