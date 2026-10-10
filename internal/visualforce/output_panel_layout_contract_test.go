package visualforce

import (
	"strings"
	"testing"

	nethtml "golang.org/x/net/html"
)

// Wrapper/placeholder rendering is captured in the asset-rendering DOM controls at
// API59/67. This retained fixture covers the same layout clauses at API67.
func TestRenderOutputPanelLayoutFamilyAPI67(t *testing.T) {
	cases := []struct {
		name     string
		pageName string
		markup   string
		panelID  string
		tag      string
		text     string
		hiddenID string
	}{
		{
			name:     "default-inline",
			pageName: "OutputPanelDefault67",
			markup:   `<apex:page><apex:outputPanel id="thePanel">My span</apex:outputPanel></apex:page>`,
			panelID:  "j_id0:thePanel",
			tag:      "span",
			text:     "My span",
		},
		{
			name:     "explicit-inline",
			pageName: "OutputPanelInline67",
			markup:   `<apex:page><apex:outputPanel id="inlinePanel" layout="inline">My span</apex:outputPanel></apex:page>`,
			panelID:  "j_id0:inlinePanel",
			tag:      "span",
			text:     "My span",
		},
		{
			name:     "none-hidden-child",
			pageName: "OutputPanelNoneHidden67",
			markup:   `<apex:page><apex:outputPanel id="nonePanel" layout="none"><apex:outputText id="hiddenChild" rendered="false" value="secret"/></apex:outputPanel></apex:page>`,
			panelID:  "j_id0:nonePanel",
			hiddenID: "j_id0:hiddenChild",
		},
		{
			name:     "explicit-block",
			pageName: "OutputPanelBlock67",
			markup:   `<apex:page><apex:outputPanel id="blockPanel" layout="block">My div</apex:outputPanel></apex:page>`,
			panelID:  "j_id0:blockPanel",
			tag:      "div",
			text:     "My div",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree, err := ParseMarkupTree(tc.markup)
			if err != nil {
				t.Fatal(err)
			}
			rendered, err := RenderMarkupTree(tree, &RenderContext{
				PageName: tc.pageName,
				PageMeta: Page{Name: tc.pageName, APIVersion: "67.0"},
			})
			if err != nil {
				t.Fatal(err)
			}
			doc, err := nethtml.Parse(strings.NewReader(rendered))
			if err != nil {
				t.Fatal(err)
			}
			panels := fieldPresentationNodes(doc, func(n *nethtml.Node) bool {
				return fieldPresentationAttr(n, "id") == tc.panelID
			})
			if tc.hiddenID != "" {
				if len(panels) != 0 {
					t.Errorf("layout none emitted panel wrapper %q: %s", tc.panelID, rendered)
				}
				hidden := fieldPresentationNodes(doc, func(n *nethtml.Node) bool {
					return fieldPresentationAttr(n, "id") == tc.hiddenID
				})
				if len(hidden) != 1 {
					t.Fatalf("hidden child %q elements = %d, want 1: %s", tc.hiddenID, len(hidden), rendered)
				}
				if hidden[0].Data != "span" {
					t.Errorf("hidden child tag = %q, want span", hidden[0].Data)
				}
				if got := fieldPresentationAttr(hidden[0], "style"); got != "display: none;" {
					t.Errorf("hidden child style = %q, want display: none;", got)
				}
				return
			}
			if len(panels) != 1 {
				t.Fatalf("panel %q elements = %d, want 1: %s", tc.panelID, len(panels), rendered)
			}
			if panels[0].Data != tc.tag {
				t.Errorf("panel tag = %q, want %q", panels[0].Data, tc.tag)
			}
			if got := fieldPresentationText(panels[0]); got != tc.text {
				t.Errorf("panel text = %q, want %q", got, tc.text)
			}
		})
	}
}
