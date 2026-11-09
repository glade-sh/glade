package project

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Native answers: `sf project deploy start --dry-run` (CLI 2.156.8) against a
// fresh scratch org at API 62.0 and 67.0, on this same synthetic layout. The
// deployed component lists were identical at both versions.
const forceIgnoreFixtureRoot = `# comment line
ignoreddir/
NameOnlyBoth.cls*
force-app/main/default/classes/AnchoredRoot.cls*
/force-app/main/default/classes/LeadSlash.cls*
**/negA/**
!**/negA/classes/NegKeep.cls*
negB/
!negB/classes/NegBKeep.cls*
classes/RelToPkg.cls*
caseclass.cls*
MetaOnly.cls-meta.xml
QMark?.cls*
Bracket[ab].cls*
anydir
sub/*
SpaceTrail.cls*
PageB.page*
Pkg2Keep.cls*
Hidden__c.field-meta.xml
`

func writeForceIgnoreFixture(t *testing.T, nestedNegB bool) string {
	t.Helper()
	root := t.TempDir()
	write := func(rel, content string) {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	class := func(dir, name string) {
		write(dir+"/"+name+".cls", "public class "+name+" {}\n")
		write(dir+"/"+name+".cls-meta.xml", `<?xml version="1.0" encoding="UTF-8"?><ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><status>Active</status></ApexClass>`)
	}
	write("sfdx-project.json", `{"packageDirectories":[{"path":"force-app","default":true},{"path":"pkg2"}],"sourceApiVersion":"62.0"}`)
	write(".forceignore", forceIgnoreFixtureRoot)
	const c = "force-app/main/default/classes"
	for _, name := range []string{"Keep0", "NameOnlyBoth", "AnchoredRoot", "LeadSlash", "RelToPkg", "CaseClass", "ContentOnly", "MetaOnly", "QMark1", "Bracketa", "Hash", "SpaceTrail"} {
		class(c, name)
	}
	class("force-app/main/ignoreddir/classes", "DirPat")
	class("force-app/main/negA/classes", "NegKeep")
	class("force-app/main/negA/classes", "NegDrop")
	class("force-app/main/negB/classes", "NegBKeep")
	class("force-app/main/anydir/deep/classes", "AnyDir")
	class("pkg2/classes", "Pkg2Local")
	class("pkg2/classes", "Pkg2Keep")
	class("pkg2/sub/classes", "StarOne")
	class("pkg2/sub/x/classes", "StarTwo")
	for _, page := range []string{"PageA", "PageB"} {
		write("force-app/main/default/pages/"+page+".page", "<apex:page/>\n")
		write("force-app/main/default/pages/"+page+".page-meta.xml", `<?xml version="1.0" encoding="UTF-8"?><ApexPage xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><label>`+page+`</label></ApexPage>`)
	}
	obj := "force-app/main/default/objects/Thing__c"
	write(obj+"/Thing__c.object-meta.xml", `<?xml version="1.0" encoding="UTF-8"?><CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Thing</label></CustomObject>`)
	for _, field := range []string{"Keep__c", "Hidden__c"} {
		write(obj+"/fields/"+field+".field-meta.xml", `<?xml version="1.0" encoding="UTF-8"?><CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>`+field+`</fullName><type>Text</type><length>10</length></CustomField>`)
	}
	// A package-level .forceignore replaces the root one for its files, and
	// its patterns are relative to its own directory.
	write("pkg2/.forceignore", "classes/Pkg2Local.cls*\n")
	write("pkg2/sub/.forceignore", "StarTwo.cls*\n")
	if nestedNegB {
		write("force-app/main/negB/.forceignore", "Nothing.cls\n")
	}
	return root
}

func baseNames(paths []string, suffix string) []string {
	var names []string
	for _, path := range paths {
		name := filepath.Base(path)
		if strings.HasSuffix(name, suffix) {
			names = append(names, strings.TrimSuffix(name, suffix))
		}
	}
	sort.Strings(names)
	return names
}

func TestForceIgnoreMatchesNativeDeploy(t *testing.T) {
	for _, tc := range []struct {
		name       string
		nestedNegB bool
		classes    []string
	}{
		{
			name:    "root and package forceignore",
			classes: []string{"ContentOnly", "Hash", "Keep0", "Pkg2Keep", "RelToPkg", "StarOne"},
		},
		{
			// The nearer negB/.forceignore governs the files below it, so the
			// root negB/ pattern no longer excludes them.
			name:       "nearer forceignore replaces outer rules",
			nestedNegB: true,
			classes:    []string{"ContentOnly", "Hash", "Keep0", "NegBKeep", "Pkg2Keep", "RelToPkg", "StarOne"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, err := Load(writeForceIgnoreFixture(t, tc.nestedNegB))
			if err != nil {
				t.Fatal(err)
			}
			if got := baseNames(p.ApexFiles, ".cls"); strings.Join(got, ",") != strings.Join(tc.classes, ",") {
				t.Fatalf("classes = %v, want %v", got, tc.classes)
			}
			if got := baseNames(p.VisualforcePageFiles, ".page"); strings.Join(got, ",") != "PageA" {
				t.Fatalf("pages = %v, want [PageA]", got)
			}
			if got := baseNames(p.FieldFiles, ".field-meta.xml"); strings.Join(got, ",") != "Keep__c" {
				t.Fatalf("fields = %v, want [Keep__c]", got)
			}
			if len(p.ForceIgnored) == 0 {
				t.Fatal("ForceIgnored is empty")
			}
			if len(p.ForceIgnoreConfigs) < 2 {
				t.Fatalf("ForceIgnoreConfigs = %v", p.ForceIgnoreConfigs)
			}
		})
	}
}

func TestForceIgnoreStaticResourceContent(t *testing.T) {
	// Native `sf project convert source` drops ignored files from inside a
	// static resource folder and keeps the rest of the resource.
	root := t.TempDir()
	for rel, content := range map[string]string{
		"sfdx-project.json": `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"62.0"}`,
		".forceignore":      "**/res/sub/\n",
		"force-app/staticresources/res.resource-meta.xml": `<?xml version="1.0" encoding="UTF-8"?><StaticResource xmlns="http://soap.sforce.com/2006/04/metadata"><contentType>application/zip</contentType></StaticResource>`,
		"force-app/staticresources/res/a.js":              "a\n",
		"force-app/staticresources/res/sub/skip.js":       "b\n",
	} {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	p, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := baseNames(p.StaticResourceFiles, ".js"); strings.Join(got, ",") != "a" {
		t.Fatalf("static resource files = %v, want [a]", got)
	}
}

func TestForceIgnoreAbsentLoadsEverything(t *testing.T) {
	root := writeForceIgnoreFixture(t, false)
	for _, rel := range []string{".forceignore", "pkg2/.forceignore", "pkg2/sub/.forceignore"} {
		if err := os.Remove(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
			t.Fatal(err)
		}
	}
	p, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(baseNames(p.ApexFiles, ".cls")); got != 21 {
		t.Fatalf("classes = %d, want 21", got)
	}
	if len(p.ForceIgnored) != 0 || len(p.ForceIgnoreConfigs) != 0 {
		t.Fatalf("ForceIgnored = %v, configs = %v", p.ForceIgnored, p.ForceIgnoreConfigs)
	}
}

// Pattern forms from the gitignore syntax the Salesforce CLI documents; the
// literal prefilter must not change any answer.
func TestForceIgnorePatternForms(t *testing.T) {
	for _, tc := range []struct {
		pattern string
		path    string
		want    bool
	}{
		{"**/__tests__/**", "__tests__/a.js", true},
		{"**/__tests__/**", "lwc/c/__tests__/a.js", true},
		{"**/__tests__/**", "lwc/c/__tests__x/a.js", false},
		{"**/jsconfig.json", "jsconfig.json", true},
		{"**/jsconfig.json", "lwc/jsconfig.json", true},
		{"a/**/b.cls", "a/b.cls", true},
		{"a/**/b.cls", "a/x/y/b.cls", true},
		{"*.page", "main/pages/P.PAGE", true},
		{"main/*.cls", "main/x/A.cls", false},
		{"main/*.cls", "main/A.cls", true},
		{"/A.cls", "x/A.cls", false},
		{"A.cls", "x/A.cls", true},
		{"[!b]x.cls", "ax.cls", true},
		{"[!b]x.cls", "bx.cls", false},
		{`\#x.cls`, "#x.cls", true},
		{"skip/", "skip", false},
		{"skip/", "skip/a.cls", true},
		{"force-app/main/Legacy*.cls*", "FORCE-APP/Main/legacyA.cls-meta.xml", true},
		{"force-app/main/Legacy*.cls*", "other/force-app/main/LegacyA.cls", false},
		{"**/flows/Old_*.flow-meta.xml", "flows/OLD_x.Flow-Meta.xml", true},
		{"**/flows/Old_*.flow-meta.xml", "flows/Old_x.flow-meta.xml.bak", false},
		{"x[ab]/C.cls", "XA/c.cls", true},
	} {
		fi := newForceIgnoreFile("/r", parseForceIgnore(tc.pattern+"\n"))
		set := &forceIgnoreSet{nearest: map[string]*forceIgnoreFile{}, loaded: map[string]*forceIgnoreFile{}}
		path := "/r/" + tc.path
		for dir := filepath.Dir(path); ; dir = filepath.Dir(dir) {
			set.nearest[dir] = fi
			if dir == "/r" {
				break
			}
		}
		if got := set.Ignored(path); got != tc.want {
			t.Errorf("%q vs %q = %v, want %v", tc.pattern, tc.path, got, tc.want)
		}
	}
}
