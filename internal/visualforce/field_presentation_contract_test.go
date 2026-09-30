package visualforce

import (
	"net/url"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	nethtml "golang.org/x/net/html"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

// TASK-11.17: metadata labels and absolute URL presentation only, not whole rows:
// visualforce-contract:67.0:rendering-pdf-profiles:input-field:metadata-widget (VF155.3)
// visualforce-contract:67.0:rendering-pdf-profiles:output-field:field-format (VF157.3)
// pages_compref_inputField.md:8-9 SHA256 fe4880d3e6c1ce9fc9d5ddc2baa8a3b7a0eb579ba98d4dd0d1e1dd1045a8ece6
// pages_compref_outputField.md:3-7 SHA256 06d1caa501b6b202fe2a78860c0ba93f7c7810968a0f05e502caf275ef398816
// Profile: API67 initial HTML GET, Account standard controller, readable fields
// and record. Lookup navigation is covered by TASK-11.16. No API59-66 interval,
// currency formatting, or Salesforce DOM claim.
func TestVisualforceFieldPresentationContractFamily(t *testing.T) {
	const (
		pageName            = "FieldPresentation67"
		nullPageName        = "NullFieldLabel67"
		outsideNullPageName = "OutsideNullFieldLabel67"
		accountID           = storage.ID("001000000000001AAA")
		ownerID             = storage.ID("005000000000001AAA")
		metadataLabel       = "Customer & account caption"
		accountName         = "Acme account"
		website             = "https://example.test/path?a=1&b=2"
		ownerName           = "Ada & <Owner>"
		plainValue          = `<b data-probe="plain">not markup</b> & "quoted"`
	)
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	pagePath := filepath.Join(root, "force-app/main/default/pages/"+pageName+".page")
	writeFile(t, pagePath, `<apex:page standardController="Account">
  <apex:form id="f">
    <apex:pageBlock>
      <apex:pageBlockSection id="fields">
        <apex:inputField id="nameField" value="{!Account.Name}"/>
        <apex:inputField id="emptyLabel" value="{!Account.Name}" label=""/>
        <apex:inputField id="customLabel" value="{!Account.Name}" label="Custom &amp; caption"/>
        <apex:inputField id="exprLabel" value="{!Account.Name}" label="{!Account.Name}"/>
        <apex:inputField id="hiddenName" value="{!Account.Name}" rendered="false"/>
        <apex:pageBlockSectionItem><apex:inputField id="sectionItemLabel" value="{!Account.Name}" label="Suppressed caption"/></apex:pageBlockSectionItem>
        <apex:outputField id="website" value="{!Account.Website}"/>
        <apex:outputField id="owner" value="{!Account.OwnerId}"/>
        <apex:outputField id="plain" value="{!Account.Description}"/>
      </apex:pageBlockSection>
    </apex:pageBlock>
    <apex:inputField value="{!Account.Name}"/>
    <apex:inputField value="{!Account.Name}"/>
    <apex:outputText id="controllerValue" value="{!Account.Name}"/>
  </apex:form>
</apex:page>`)
	writeFile(t, pagePath+"-meta.xml", `<?xml version="1.0" encoding="UTF-8"?>
<ApexPage xmlns="http://soap.sforce.com/2006/04/metadata">
  <apiVersion>67.0</apiVersion><label>Field Presentation 67</label>
</ApexPage>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/"+nullPageName+".page"),
		`<apex:page standardController="Account"><apex:pageBlock><apex:pageBlockSection><apex:inputField value="{!Account.Name}" label="{!NULL}"/></apex:pageBlockSection></apex:pageBlock></apex:page>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/"+nullPageName+".page-meta.xml"),
		`<?xml version="1.0" encoding="UTF-8"?><ApexPage xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion><label>Null Field Label 67</label></ApexPage>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/"+outsideNullPageName+".page"),
		`<apex:page standardController="Account"><apex:form><apex:inputField value="{!Account.Name}" label="{!NULL}"/></apex:form></apex:page>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/"+outsideNullPageName+".page-meta.xml"),
		`<?xml version="1.0" encoding="UTF-8"?><ApexPage xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion><label>Outside Null Field Label 67</label></ApexPage>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	page, ok := idx.Page(pageName)
	if !ok || page.APIVersion != "67.0" || page.StandardController != "Account" {
		t.Fatalf("wrong page profile: found=%v api=%q controller=%q", ok, page.APIVersion, page.StandardController)
	}
	owner := storage.Record{ID: ownerID, Object: "User", Fields: map[string]storage.Value{
		"Name": storage.StringValue(ownerName),
	}}
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Account", KeyPrefix: "001", Fields: map[string]storage.Field{
			"Name":        {APIName: "Name", Label: metadataLabel, Type: storage.FieldString, Accessible: storage.BoolFlag(true), Updateable: storage.BoolFlag(true)},
			"Website":     {APIName: "Website", Label: "Website", Type: storage.FieldString, DisplayType: "URL", Accessible: storage.BoolFlag(true)},
			"OwnerId":     {APIName: "OwnerId", Label: "Owner", Type: storage.FieldReference, DisplayType: "REFERENCE", ReferenceTo: []string{"User"}, RelationshipName: "Owner", Accessible: storage.BoolFlag(true)},
			"Description": {APIName: "Description", Label: "Description", Type: storage.FieldString, Accessible: storage.BoolFlag(true)},
		}},
		Records: map[storage.ID]storage.Record{accountID: {
			ID: accountID, Object: "Account", System: storage.SystemFields{OwnerID: ownerID}, Fields: map[string]storage.Value{
				"Name": storage.StringValue(accountName), "Website": storage.StringValue(website),
				"OwnerId": storage.IDValue(ownerID), "Description": storage.StringValue(plainValue),
			}, ParentRelationships: map[string]storage.Record{"Owner": owner},
		}},
	}
	org.Objects["User"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "User", KeyPrefix: "005", Fields: map[string]storage.Field{
			"Name": {APIName: "Name", Label: "Full Name", Type: storage.FieldString, Accessible: storage.BoolFlag(true)},
		}}, Records: map[storage.ID]storage.Record{ownerID: owner},
	}
	render := func(t *testing.T) *nethtml.Node {
		t.Helper()
		machine := vm.New(nil)
		machine.SetOrg(&org)
		authorizeFieldRenderingFixture(t, &org, machine, "Website", "OwnerId", "Description")
		authorizeFieldRenderingOwnerNameFixture(t, &org, owner)
		result, err := RenderPage(PageRenderRequest{Project: p, VFIndex: idx, Org: &org,
			Machine: machine, PageName: pageName, PageURL: "/apex/" + pageName + "?id=" + string(accountID)})
		if err != nil {
			t.Fatal(err)
		}
		if result.Error != nil || result.Redirect || result.RenderAs == "pdf" {
			t.Fatalf("expected initial HTML render: error=%v redirect=%v renderAs=%q", result.Error, result.Redirect, result.RenderAs)
		}
		doc, err := nethtml.Parse(strings.NewReader(result.HTML))
		if err != nil {
			t.Fatal(err)
		}
		return doc
	}
	doc := render(t)
	byID := func(t *testing.T, id string) *nethtml.Node {
		t.Helper()
		matches := fieldPresentationNodes(doc, func(n *nethtml.Node) bool { return fieldPresentationAttr(n, "id") == id })
		if len(matches) != 1 {
			t.Fatalf("expected one generated ID %q, got %d", id, len(matches))
		}
		return matches[0]
	}
	t.Run("metadata_label_inside_section", func(t *testing.T) {
		section := byID(t, "j_id0:f:fields")
		if !strings.Contains(fieldPresentationText(section), metadataLabel) {
			t.Errorf("VF155.3: section lacks metadata label %q", metadataLabel)
		}
		labels := fieldPresentationNodes(section, func(n *nethtml.Node) bool { return n.Data == "label" })
		if len(labels) != 3 {
			t.Fatalf("expected metadata and two explicit labels only, got %d", len(labels))
		}
		if fieldPresentationAttr(labels[0], "for") != "j_id0:f:nameField" || fieldPresentationText(labels[0]) != metadataLabel {
			t.Error("metadata label does not preserve its text and named input association")
		}
		if fieldPresentationAttr(labels[1], "for") != "j_id0:f:customLabel" || fieldPresentationText(labels[1]) != "Custom & caption" {
			t.Error("explicit label did not override metadata or preserve its input association")
		}
		if fieldPresentationAttr(labels[2], "for") != "j_id0:f:exprLabel" || fieldPresentationText(labels[2]) != accountName {
			t.Error("expression label did not override metadata with the evaluated value")
		}
		if len(fieldPresentationNodes(section, func(n *nethtml.Node) bool {
			return n.Data == "label" && fieldPresentationAttr(n, "for") == "j_id0:f:emptyLabel"
		})) != 0 {
			t.Error("label-empty input unexpectedly displayed a metadata label")
		}
		if len(fieldPresentationNodes(section, func(n *nethtml.Node) bool { return fieldPresentationAttr(n, "id") == "j_id0:f:hiddenName" })) != 0 {
			t.Error("rendered=false input unexpectedly appeared")
		}
	})
	t.Run("null_label_expression_is_error", func(t *testing.T) {
		machine := vm.New(nil)
		machine.SetOrg(&org)
		authorizeFieldRenderingFixture(t, &org, machine, "Website", "OwnerId", "Description")
		result, err := RenderPage(PageRenderRequest{Project: p, VFIndex: idx, Org: &org,
			Machine: machine, PageName: nullPageName, PageURL: "/apex/" + nullPageName + "?id=" + string(accountID)})
		if err == nil || result.Error == nil ||
			!strings.Contains(result.Error.Message, "inputField label") || !strings.Contains(result.Error.Message, "null") {
			t.Fatalf("expected source-required null-label error; err=%v renderError=%v", err, result.Error)
		}
	})
	t.Run("section_item_suppresses_field_label", func(t *testing.T) {
		labels := fieldPresentationNodes(doc, func(n *nethtml.Node) bool {
			return n.Data == "label" && fieldPresentationAttr(n, "for") == "j_id0:f:sectionItemLabel"
		})
		if len(labels) != 0 {
			t.Fatal("pageBlockSectionItem unexpectedly emitted its child inputField label")
		}
	})
	t.Run("null_label_outside_section_is_error", func(t *testing.T) {
		machine := vm.New(nil)
		machine.SetOrg(&org)
		authorizeFieldRenderingFixture(t, &org, machine, "Website", "OwnerId", "Description")
		result, err := RenderPage(PageRenderRequest{Project: p, VFIndex: idx, Org: &org,
			Machine: machine, PageName: outsideNullPageName, PageURL: "/apex/" + outsideNullPageName + "?id=" + string(accountID)})
		if err == nil || result.Error == nil ||
			!strings.Contains(result.Error.Message, "inputField label") || !strings.Contains(result.Error.Message, "null") {
			t.Fatalf("expected source-required outside-section null-label error; err=%v renderError=%v", err, result.Error)
		}
	})
	for _, tc := range []struct{ name, id, text string }{
		{"url_link", "j_id0:f:website", website},
	} {
		t.Run(tc.name, func(t *testing.T) {
			field := byID(t, tc.id)
			links := fieldPresentationNodes(field, func(n *nethtml.Node) bool { return n.Data == "a" })
			if len(links) != 1 {
				t.Fatalf("VF157.3: expected one field link, got %d; text=%q", len(links), fieldPresentationText(field))
			}
			link := links[0]
			if got := fieldPresentationText(link); got != tc.text {
				t.Errorf("link text=%q, want %q", got, tc.text)
			}
			href := fieldPresentationAttr(link, "href")
			target, err := url.Parse(href)
			if err != nil || href == "" || strings.HasPrefix(href, "#") {
				t.Fatalf("unusable field link %q: %v", href, err)
			}
			if target.Scheme != "" && target.Scheme != "https" && target.Scheme != "http" {
				t.Errorf("unsafe field link scheme %q", target.Scheme)
			}
			if tc.name == "url_link" && href != website {
				t.Errorf("URL target=%q, want %q", href, website)
			}
			if len(fieldPresentationNodes(link, func(n *nethtml.Node) bool { return n.Data == "owner" })) != 0 {
				t.Error("field text became nested markup")
			}
		})
	}
	t.Run("lookup_has_authorized_local_link", func(t *testing.T) {
		field := byID(t, "j_id0:f:owner")
		if got := fieldPresentationText(field); got != ownerName {
			t.Errorf("lookup text=%q, want %q", got, ownerName)
		}
		links := fieldPresentationNodes(field, func(n *nethtml.Node) bool { return n.Data == "a" })
		if len(links) != 1 {
			t.Fatalf("lookup links=%d, want one authorized local destination", len(links))
		}
		if got, want := fieldPresentationAttr(links[0], "href"), "/record/User/"+string(ownerID); got != want {
			t.Errorf("lookup href=%q, want %q", got, want)
		}
		if got := fieldPresentationText(links[0]); got != ownerName {
			t.Errorf("lookup link text=%q, want %q", got, ownerName)
		}
	})
	t.Run("plain_value_escaping", func(t *testing.T) {
		plain := byID(t, "j_id0:f:plain")
		if got := fieldPresentationText(plain); got != plainValue {
			t.Errorf("plain text=%q, want %q", got, plainValue)
		}
		if len(fieldPresentationNodes(plain, func(n *nethtml.Node) bool { return fieldPresentationAttr(n, "data-probe") == "plain" })) != 0 {
			t.Error("plain outputField created markup instead of escaped text")
		}
	})
	t.Run("controller_value_and_named_id_stability", func(t *testing.T) {
		if got := fieldPresentationText(byID(t, "j_id0:f:controllerValue")); got != accountName {
			t.Errorf("standard-controller value=%q, want %q", got, accountName)
		}
		if got := fieldPresentationAttr(byID(t, "j_id0:f:nameField"), "value"); got != accountName {
			t.Errorf("input value=%q, want %q", got, accountName)
		}
		// The two unnamed input IDs are an existing, separate behavior. This
		// batch preserves only the explicitly named input's qualified client ID.
		inputIDs := func(tree *nethtml.Node) []string {
			var ids []string
			for _, n := range fieldPresentationNodes(tree, func(n *nethtml.Node) bool {
				return n.Data == "input" && fieldPresentationAttr(n, "id") == "j_id0:f:nameField"
			}) {
				ids = append(ids, fieldPresentationAttr(n, "id"))
			}
			return ids
		}
		ids := inputIDs(doc)
		if len(ids) != 1 {
			t.Fatalf("expected one explicitly named input, got %v", ids)
		}
		if again := inputIDs(render(t)); !reflect.DeepEqual(ids, again) {
			t.Errorf("named input ID changed between initial renders: %v -> %v", ids, again)
		}
	})
	t.Run("url_href_safety", func(t *testing.T) {
		record := org.Objects["Account"].Records[accountID]
		original := record.Fields["Website"]
		t.Cleanup(func() { record.Fields["Website"] = original })
		for _, tc := range []struct {
			name, value string
			link        bool
		}{
			{"http", "http://example.test/path", true},
			{"escaped_attribute", `https://example.test/?q="quoted"&a=1`, true},
			{"javascript", "javascript:alert(1)", false},
			{"mixed_case_script", "JaVaScRiPt:alert(1)", false},
			{"data", "data:text/html,<svg onload=alert(1)>", false},
			{"vbscript", "vbscript:msgbox(1)", false},
			{"scheme_relative", "//example.test/path", false},
			{"relative", "/record/path", false},
			{"missing_host", "https:///missing-host", false},
			{"control_character", "https://example.test/\tbad", false},
			{"backslash", `https://example.test\@other.test/`, false},
		} {
			record.Fields["Website"] = storage.StringValue(tc.value)
			fields := fieldPresentationNodes(render(t), func(n *nethtml.Node) bool { return fieldPresentationAttr(n, "id") == "j_id0:f:website" })
			if len(fields) != 1 {
				t.Fatalf("%s: missing named URL output", tc.name)
			}
			field := fields[0]
			links := fieldPresentationNodes(field, func(n *nethtml.Node) bool { return n.Data == "a" })
			if tc.link {
				if len(links) != 1 || fieldPresentationAttr(links[0], "href") != tc.value {
					t.Errorf("%s: safe URL did not retain its exact decoded href", tc.name)
				}
			} else if len(links) != 0 {
				t.Errorf("%s: unsafe or unsupported URL became a link", tc.name)
			}
			if got := fieldPresentationText(field); got != tc.value {
				t.Errorf("%s: text=%q, want %q", tc.name, got, tc.value)
			}
			if len(fieldPresentationNodes(field, func(n *nethtml.Node) bool { return n.Data == "svg" || fieldPresentationAttr(n, "onload") != "" })) != 0 {
				t.Errorf("%s: URL text became executable markup", tc.name)
			}
		}
	})
}

func fieldPresentationNodes(root *nethtml.Node, match func(*nethtml.Node) bool) []*nethtml.Node {
	var out []*nethtml.Node
	var visit func(*nethtml.Node)
	visit = func(n *nethtml.Node) {
		if n.Type == nethtml.ElementNode && match(n) {
			out = append(out, n)
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(root)
	return out
}

func fieldPresentationAttr(n *nethtml.Node, name string) string {
	for _, attr := range n.Attr {
		if attr.Key == name {
			return attr.Val
		}
	}
	return ""
}

func fieldPresentationText(root *nethtml.Node) string {
	var text strings.Builder
	var visit func(*nethtml.Node)
	visit = func(n *nethtml.Node) {
		if n.Type == nethtml.TextNode {
			text.WriteString(n.Data)
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(root)
	return text.String()
}
