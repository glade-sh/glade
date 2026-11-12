package visualforce_test

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/visualforce"
)

// Rows of testdata/vf_component_ids: whether the org's compile accepts a page
// or component whose controls repeat an id. A custom component's body and a
// pageBlockSectionItem each prefix the ids inside them, so the same id there
// is not a duplicate; two controls of one id in one scope still are, whether or
// not their rendered attributes exclude each other. A rejected page or
// component is reported and the project's other pages stay registered.
func TestPageNamesNativeComponentIDRows(t *testing.T) {
	type row struct{ outcome, value string }
	cases := map[string][2]string{} // case -> where, markup
	file, err := os.Open("testdata/vf_component_ids/cases.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	scan := bufio.NewScanner(file)
	scan.Buffer(make([]byte, 1<<20), 1<<20)
	scan.Scan()
	var order []string
	for scan.Scan() {
		parts := strings.SplitN(scan.Text(), "\t", 3)
		cases[parts[0]] = [2]string{parts[1], parts[2]}
		order = append(order, parts[0])
	}
	native := map[string]map[string]row{} // "api/artifact" -> case -> row
	data, err := os.ReadFile("testdata/vf_component_ids/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n")[1:] {
		f := strings.SplitN(line, "\t", 5)
		key := f[0] + "/" + f[1]
		if native[key] == nil {
			native[key] = map[string]row{}
		}
		native[key][f[2]] = row{f[3], f[4]}
	}
	meta := func(tag, api, label string) string {
		return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?><%s xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><label>%s</label></%s>`, tag, api, label, tag)
	}
	for key, rows := range native {
		api, artifact, _ := strings.Cut(key, "/")
		t.Run(key, func(t *testing.T) {
			root := t.TempDir()
			write := func(name, source string) { writePageNameFixture(t, root, filepath.FromSlash(name), source) }
			write("sfdx-project.json", fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			const base = "force-app/main/default/"
			write(base+"components/CedarWrap.component", `<apex:component><div class="wrap"><apex:componentBody/></div></apex:component>`)
			write(base+"components/CedarWrap.component-meta.xml", meta("ApexComponent", artifact, "CedarWrap"))
			write(base+"classes/CedarListCtl.cls", "public class CedarListCtl { public class Row { public String Name { get; set; } public String Label { get; set; } } public List<Row> rows { get; set; } public List<String> items { get; set; } }")
			write(base+"classes/CedarListCtl.cls-meta.xml", `<?xml version="1.0" encoding="UTF-8"?><ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			for _, object := range []string{"CedarObj__c", "CedarTabObj__c"} {
				write(base+"objects/"+object+"/"+object+".object-meta.xml", `<?xml version="1.0" encoding="UTF-8"?><CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Cedar</label><pluralLabel>Cedars</pluralLabel><deploymentStatus>Deployed</deploymentStatus><sharingModel>ReadWrite</sharingModel><nameField><label>Name</label><type>Text</type></nameField></CustomObject>`)
			}
			write(base+"tabs/CedarTabObj__c.tab-meta.xml", `<?xml version="1.0" encoding="UTF-8"?><CustomTab xmlns="http://soap.sforce.com/2006/04/metadata"><customObject>true</customObject><motif>Custom43: Jewel</motif></CustomTab>`)
			// An unrelated page that must stay registered whatever the rows say.
			write(base+"pages/CedarBystander.page", `<apex:page>Bystander</apex:page>`)
			write(base+"pages/CedarBystander.page-meta.xml", meta("ApexPage", artifact, "CedarBystander"))
			want := []string{"CedarBystander"}
			for _, name := range order {
				where, markup := cases[name][0], cases[name][1]
				page := "CedarP" + name
				if where == "page" {
					write(base+"pages/"+page+".page", `<apex:page><div>`+markup+`</div></apex:page>`)
				} else if where == "ctl" {
					write(base+"pages/"+page+".page", `<apex:page controller="CedarListCtl"><div>`+markup+`</div></apex:page>`)
				} else {
					comp := "CedarC" + name
					write(base+"components/"+comp+".component", `<apex:component>`+markup+`</apex:component>`)
					write(base+"components/"+comp+".component-meta.xml", meta("ApexComponent", artifact, comp))
					write(base+"pages/"+page+".page", `<apex:page><div><c:`+comp+`/></div></apex:page>`)
				}
				write(base+"pages/"+page+".page-meta.xml", meta("ApexPage", artifact, page))
				if rows[name].outcome == "ids" || rows[name].outcome == "exception" {
					want = append(want, page) // an exception is the page compiling and failing at render
				}
			}
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			names, rejected := visualforce.PageNamesWithDiagnostics(p, nil)
			slices.Sort(want)
			if !slices.Equal(names, want) {
				t.Fatalf("registered pages = %v, want %v; rejected %+v", names, want, rejected)
			}
			for _, name := range order {
				if rows[name].outcome != "error" {
					continue
				}
				found := false
				for _, r := range rejected {
					if strings.Contains(r.File, "CedarP"+name+".page") || strings.Contains(r.File, "CedarC"+name+".component") {
						found = true
					}
				}
				if !found {
					t.Errorf("%s: native rejects (%s) but Glade reports no rejection", name, rows[name].value)
				}
			}
		})
	}
}

// Rows Ns* of testdata/vf_component_ids (Developer Edition org with a
// namespace): source refers to its own custom objects, fields and Apex types
// without the prefix. A component attribute may name an own custom object (also
// as a list), interface, enum or class; a page binds an own lookup by its
// unprefixed field or relationship name. A name no source declares is rejected
// with the org's message.
func TestPageNamesNativeNamespacedRows(t *testing.T) {
	const namespace = "gladens001"
	file, err := os.Open("testdata/vf_component_ids/cases-ns.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	scan := bufio.NewScanner(file)
	scan.Buffer(make([]byte, 1<<20), 1<<20)
	scan.Scan()
	type nsCase struct{ name, where, markup string }
	var cases []nsCase
	for scan.Scan() {
		parts := strings.SplitN(scan.Text(), "\t", 3)
		cases = append(cases, nsCase{parts[0], parts[1], strings.ReplaceAll(parts[2], "%NS%", namespace)})
	}
	data, err := os.ReadFile("testdata/vf_component_ids/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	native := map[string]map[string][2]string{} // "api/artifact" -> case -> outcome, value
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n")[1:] {
		f := strings.SplitN(line, "\t", 5)
		if !strings.HasPrefix(f[2], "Ns") {
			continue
		}
		key := f[0] + "/" + f[1]
		if native[key] == nil {
			native[key] = map[string][2]string{}
		}
		native[key][f[2]] = [2]string{f[3], f[4]}
	}
	if len(native) == 0 {
		t.Fatal("no namespaced rows")
	}
	const h = `<?xml version="1.0" encoding="UTF-8"?>`
	const xmlns = ` xmlns="http://soap.sforce.com/2006/04/metadata"`
	for key, rows := range native {
		api, artifact, _ := strings.Cut(key, "/")
		t.Run(key, func(t *testing.T) {
			root := t.TempDir()
			write := func(name, source string) { writePageNameFixture(t, root, filepath.FromSlash(name), source) }
			write("sfdx-project.json", fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"namespace":%q,"sourceApiVersion":%q}`, namespace, api))
			const base = "force-app/main/default/"
			class := func(name, body string) {
				write(base+"classes/"+name+".cls", body)
				write(base+"classes/"+name+".cls-meta.xml", h+`<ApexClass`+xmlns+`><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			}
			class("CedarIface", "public interface CedarIface { String name(); }")
			class("CedarEnum", "public enum CedarEnum { A, B }")
			class("CedarCls", "public class CedarCls { public String s { get; set; } }")
			class("CedarExt", "public class CedarExt { public CedarExt(ApexPages.StandardController c) {} }")
			class("CedarCtl", "public class CedarCtl { public List<Account> accs { get; set; } public CedarCtl() { accs = new List<Account>{ new Account(Name = 'x') }; } }")
			write(base+"objects/CedarObj__c/CedarObj__c.object-meta.xml", h+`<CustomObject`+xmlns+`><label>Cedar Obj</label><pluralLabel>Cedar Objs</pluralLabel><deploymentStatus>Deployed</deploymentStatus><sharingModel>ReadWrite</sharingModel><nameField><label>Name</label><type>Text</type></nameField></CustomObject>`)
			write(base+"objects/Account/fields/CedarRel__c.field-meta.xml", h+`<CustomField`+xmlns+`><fullName>CedarRel__c</fullName><label>Cedar Rel</label><referenceTo>Contact</referenceTo><relationshipLabel>Cedar Rels</relationshipLabel><relationshipName>CedarRel</relationshipName><type>Lookup</type><deleteConstraint>SetNull</deleteConstraint></CustomField>`)
			meta := func(tag, label string) string {
				return h + `<` + tag + xmlns + `><apiVersion>` + artifact + `</apiVersion><label>` + label + `</label></` + tag + `>`
			}
			var want []string
			for _, c := range cases {
				page := "CedarNP" + c.name
				if c.where == "comp" {
					comp := "CedarNC" + c.name
					write(base+"components/"+comp+".component", `<apex:component>`+c.markup+`</apex:component>`)
					write(base+"components/"+comp+".component-meta.xml", meta("ApexComponent", comp))
					write(base+"pages/"+page+".page", `<apex:page><div><c:`+comp+`/></div></apex:page>`)
				} else if c.where == "std" {
					// An extension puts project source on the page, which Glade needs to check field references.
					write(base+"pages/"+page+".page", `<apex:page standardController="Account" extensions="CedarExt"><div>`+c.markup+`</div></apex:page>`)
				} else {
					write(base+"pages/"+page+".page", `<apex:page controller="CedarCtl"><div>`+c.markup+`</div></apex:page>`)
				}
				write(base+"pages/"+page+".page-meta.xml", meta("ApexPage", page))
				if rows[c.name][0] != "error" { // ids or exception: the page compiled
					want = append(want, page)
				}
			}
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			names, rejected := visualforce.PageNamesWithDiagnostics(p, nil)
			slices.Sort(want)
			if !slices.Equal(names, want) {
				t.Fatalf("registered pages = %v, want %v; rejected %+v", names, want, rejected)
			}
		})
	}
}
