package sema

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// Covers lifecycle, override, Batchable, nested-type, duplicate-object,
// schema-enrichment and SOQL-inferred-object paths on one shared index.
// Run under -race.
func TestAnalyzeBroadConstructsConcurrentOnSharedIndex(t *testing.T) {
	root := t.TempDir()
	classes := map[string]string{
		"Base": `public virtual class Base { public virtual void go(String s, Integer n) {} public class Inner { public Inner other; } }`,
		"Kid":  `public class Kid extends Base { public override void go(String s, Integer n) {} public void x(Base.Inner i) { Inner j = i; } }`,
		"Bad":  `public class Bad extends Base { @Override public void gone(Type t, Inner i) {} }`,
		"Batch": `public class Batch implements Database.Batchable<Account>, Iterable<Account>, Iterator<Account> {
  public Database.QueryLocator start(Database.BatchableContext bc) { return null; }
  public void execute(Database.BatchableContext bc, List<Account> rows) { for (Account a : rows) { a.Foo__c = 'x'; } }
}`,
		"Cb": `public class Cb implements EventBus.EventPublishSuccessCallback { }`,
		"Q":  `public class Q { public void m() { List<Thing__c> l = [SELECT Id, Zed__c, Par__r.Name FROM Thing__c WHERE Zed__c = 'x']; List<Meta__mdt> m = [SELECT Label, DeveloperName, Extra__c FROM Meta__mdt]; Setting__c s = Setting__c.getOrgDefaults(); Integer q = s.SetupOwnerId == null ? 1 : 0; Typed__c t = new Typed__c(RecordTypeId = null); Account a = [SELECT Id, Custom__c FROM Account]; } }`,
	}
	var files []string
	for name, body := range classes {
		p := filepath.Join(root, name+".cls")
		os.WriteFile(p, []byte(body), 0o600)
		files = append(files, p)
	}
	nf := func(f ...schema.Field) []schema.Field {
		o := make([]schema.Field, len(f), len(f)+8)
		copy(o, f)
		return o
	}
	idx := typesys.Build(project.Project{Root: root, SourceAPIVersion: "67.0", ApexFiles: files}, schema.Schema{Objects: []schema.Object{
		{Name: "Account", Fields: nf(schema.Field{Name: "Name", Type: "Text"})},
		{Name: "Account", Fields: nf(schema.Field{Name: "Other", Type: ""})},
		{Name: "Thing__c", Fields: nf(schema.Field{Name: "Zed__c"}, schema.Field{Name: "Par__c", Type: "Lookup"}), EnableSharing: true},
		{Name: "Meta__mdt", Fields: nf(schema.Field{Name: "Label"})},
		{Name: "Setting__c", CustomSettingsType: "Hierarchy", Fields: nf(schema.Field{Name: "SetupOwnerId", Type: "Text"})},
		{Name: "Typed__c", Fields: nf(schema.Field{Name: "RecordTypeId", Type: "Text"})},
	}})
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			AnalyzeWithOptions(idx, AnalyzeOptions{Diagnostics: true, ExportTypes: true})
			AnalyzeAnonymous(idx, "Kid k = new Kid(); k.go('a',1); List<Q__c> z = [SELECT Id, Nw__c FROM Q__c];", "67.0")
		}()
	}
	wg.Wait()
}
