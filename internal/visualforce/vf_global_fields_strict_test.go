package visualforce_test

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/visualforce"
)

func strictLoad(t *testing.T, page string) error {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "force-app/main/default/pages")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"sfdx-project.json":                            `{"packageDirectories":[{"path":"force-app","default":true}],"namespace":"","sourceApiVersion":"67.0"}`,
		"force-app/main/default/pages/S.page":          page,
		"force-app/main/default/pages/S.page-meta.xml": `<?xml version="1.0" encoding="UTF-8"?>` + "\n" + `<ApexPage xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion><label>S</label></ApexPage>` + "\n",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	_, err = visualforce.LoadProject(p)
	return err
}

// Every $User and $Organization field that exists natively stays accepted on a
// controller-less page; only the captured absent fields and unknown names fail.
func TestControllerlessGlobalFieldsAcceptNativeFields(t *testing.T) {
	hiddenUser := map[string]bool{"emailencodingkey": true, "digestfrequency": true, "defaultgroupnotificationfrequency": true, "portalrole": true}
	hiddenOrg := map[string]bool{"organizationtype": true, "languagelocalekey": true}
	for _, tc := range []struct {
		root, object string
		hidden       map[string]bool
	}{{"$User", "User", hiddenUser}, {"$Organization", "Organization", hiddenOrg}} {
		definition, ok := storage.StandardObjectDefinition(tc.object)
		if !ok {
			t.Fatalf("no %s definition", tc.object)
		}
		var names []string
		for name := range definition.Fields {
			names = append(names, name)
		}
		sort.Strings(names)
		var page strings.Builder
		page.WriteString(`<apex:page showHeader="false">`)
		count := 0
		for _, name := range names {
			if tc.hidden[strings.ToLower(name)] {
				continue
			}
			expr := tc.root + "." + name
			if definition.Fields[name].Type == storage.FieldPicklist {
				expr = fmt.Sprintf("TEXT(%s)", expr)
			}
			fmt.Fprintf(&page, `<apex:outputText value="{!%s}"/>`, expr)
			count++
		}
		page.WriteString(`</apex:page>`)
		if err := strictLoad(t, page.String()); err != nil {
			t.Fatalf("%s: native fields rejected: %v", tc.root, err)
		}
		t.Logf("%s: %d fields accepted", tc.root, count)
	}
	for _, accepted := range []string{
		"$User.UITheme", "$User.UIThemeDisplayed", "$User.Custom_Field__c", "$User.Manager.Name", "$User.Profile.Name", "$Organization.Custom__c",
	} {
		if err := strictLoad(t, `<apex:page><apex:outputText value="{!`+accepted+`}"/></apex:page>`); err != nil {
			t.Errorf("%s rejected: %v", accepted, err)
		}
	}
	for expr, want := range map[string]string{
		"$User.NoSuchField":                 "Field NoSuchField does not exist. Check spelling.",
		"$Organization.OrganizationType":    "Field OrganizationType does not exist. Check spelling.",
		"$Organization.LanguageLocaleKey":   "Field LanguageLocaleKey does not exist. Check spelling.",
		"$Organization.NoSuchField":         "Field NoSuchField does not exist. Check spelling.",
		"ISPICKVAL($User.NoSuchField, 'x')": "Field NoSuchField does not exist. Check spelling.",
	} {
		err := strictLoad(t, `<apex:page><apex:outputText value="{!`+expr+`}"/></apex:page>`)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: error=%v, want %q", expr, err, want)
		}
	}
}

// An entity in plain attribute text, in markup text, or inside a string literal
// stays accepted; an entity in the expression outside a string literal is the
// native syntax error.
func TestEntityInExpressionOnlyRejectedOutsideStringLiterals(t *testing.T) {
	for _, page := range []string{
		`<apex:page><apex:outputText value="a &amp; b &quot;c&quot;"/></apex:page>`,
		`<apex:page><apex:outputText value="a &amp; {!'x'} &lt;"/></apex:page>`,
		`<apex:page>&amp; {!'x'} &lt; &quot;</apex:page>`,
		`<apex:page><apex:outputText value="{!'a&amp;b'}"/></apex:page>`,
		`<apex:page><apex:outputText value="{!IF(true, 'a&quot;b', 'c')} &amp; {!'y'}"/></apex:page>`,
		`<apex:page><!-- {!&quot;a&quot;} --><apex:outputText value='{!"a"}'/></apex:page>`,
		`<apex:page><apex:repeat value='{!["1"="1","2"="2"]}' var="q">[{!q}]</apex:repeat></apex:page>`,
	} {
		if err := strictLoad(t, page); err != nil {
			t.Errorf("%s rejected: %v", page, err)
		}
	}
	for page, want := range map[string]string{
		`<apex:page><apex:outputText value="{!&quot;a&quot;}"/></apex:page>`:              "Syntax error.  Found '&'",
		`<apex:page>{!&quot;a&quot;}</apex:page>`:                                         "Syntax error.  Found '&'",
		`<apex:page><apex:outputText value="{!IF(true,&apos;a&apos;,'b')}"/></apex:page>`: "Syntax error.  Found '&'",
		`<apex:page><apex:outputText value="{!1 == &#34;a&#34;}"/></apex:page>`:           "Syntax error.  Found '&'",
		`<apex:page><apex:outputText value="{!true &amp;&amp; false}"/></apex:page>`:      "Syntax error",
		`<apex:page><apex:outputText value="{!1 &lt; 2}"/></apex:page>`:                   "Syntax error",
	} {
		err := strictLoad(t, page)
		if err == nil || err.Error() != want {
			t.Errorf("%s: error=%v, want %q", page, err, want)
		}
	}
}
