package sema

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestAnalyzeCapturedMultiCurrencySchema(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"sfdx-project.json": `{"sourceApiVersion":"53.0","packageDirectories":[{"path":"force-app","default":true}]}`,
		"glade.yml":         "org:\n  features: [MultiCurrency]\n",
		"force-app/main/default/classes/CurrencySemanticProbe53.cls": `public class CurrencySemanticProbe53 {
 public static String readCurrency(Id parentId) {
  GladeCurrencyParent__c row = [SELECT Id,CurrencyIsoCode FROM GladeCurrencyParent__c WHERE Id=:parentId];
  return row.CurrencyIsoCode;
 }
}`,
		"force-app/main/default/classes/CurrencySemanticProbe53.cls-meta.xml":                          `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>53.0</apiVersion><status>Active</status></ApexClass>`,
		"force-app/main/default/objects/GladeCurrencyParent__c/GladeCurrencyParent__c.object-meta.xml": "<CustomObject xmlns=\"http://soap.sforce.com/2006/04/metadata\"><label>Glade Currency Parent</label><pluralLabel>Glade Currency Parents</pluralLabel><nameField><label>Name</label><type>Text</type></nameField><deploymentStatus>Deployed</deploymentStatus><sharingModel>ReadWrite</sharingModel></CustomObject>\n",
		"force-app/main/default/objects/GladeCurrencyParent__c/fields/Amount__c.field-meta.xml":        "<CustomField xmlns=\"http://soap.sforce.com/2006/04/metadata\"><fullName>Amount__c</fullName><label>Amount</label><type>Currency</type><precision>18</precision><scale>2</scale><required>false</required></CustomField>\n",
	}
	for name, contents := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		writeSemaFile(t, path, contents)
	}
	load := func() (typesys.Index, project.Project) {
		t.Helper()
		p, err := project.Load(root)
		if err != nil {
			t.Fatal(err)
		}
		s, err := schema.LoadProject(p)
		if err != nil {
			t.Fatal(err)
		}
		return typesys.Build(p, s), p
	}
	assertKnown := func(index typesys.Index) {
		t.Helper()
		if result := Analyze(index); result.HasErrors() {
			t.Fatalf("enabled feature lost in query/member checking: %#v", result.Diagnostics)
		}
		const anonymousSource = `GladeCurrencyParent__c row = [SELECT Id,CurrencyIsoCode FROM GladeCurrencyParent__c LIMIT 1]; String code = row.CurrencyIsoCode;`
		if result := AnalyzeAnonymous(index, anonymousSource, "53.0"); result.HasErrors() {
			t.Fatalf("enabled feature lost in API53 anonymous checking: %#v", result.Diagnostics)
		}
		if result := AnalyzeAnonymous(index, anonymousSource, "67.0"); result.HasErrors() {
			t.Fatalf("enabled feature lost in API67 anonymous checking: %#v", result.Diagnostics)
		}
	}
	on, p := load()
	before, err := json.Marshal(on)
	if err != nil {
		t.Fatal(err)
	}
	assertKnown(on)
	// A captured index remains valid under later config edits, while a fresh
	// index must rebind the changed feature set rather than reuse its identity.
	writeSemaFile(t, filepath.Join(root, "glade.yml"), "org:\n  features: []\n")
	assertKnown(on)
	if typesys.MatchesProjectIdentity(on, p) {
		t.Fatal("feature change reused old project identity")
	}
	off, _ := load()
	if typesys.SameProjectIdentity(on, off) {
		t.Fatal("feature sets share an index identity")
	}
	result := Analyze(off)
	found := false
	for _, item := range result.Diagnostics {
		if item.Code == "GLADESEMA_QUERY_FIELD" && strings.Contains(item.Message, "GladeCurrencyParent__c.CurrencyIsoCode") {
			found = true
		}
	}
	if !found {
		t.Fatalf("disabled feature unexpectedly accepted query: %#v", result.Diagnostics)
	}
	after, err := json.Marshal(on)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("semantic enrichment mutated captured source/index")
	}
	// Snapshot serialization preserves the feature contract without consulting
	// the now-disabled live config.
	var restored typesys.Index
	if err := json.Unmarshal(before, &restored); err != nil {
		t.Fatal(err)
	}
	assertKnown(restored)
}
