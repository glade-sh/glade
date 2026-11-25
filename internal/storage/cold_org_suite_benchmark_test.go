package storage_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// BenchmarkColdOrgTestSuite includes pure, DML, describe and inventory reads.
// A project schema overlay forces construction instead of the shared shaped seed.
func BenchmarkColdOrgTestSuite(b *testing.B) {
	root := b.TempDir()
	write := func(name, content string) {
		b.Helper()
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			b.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			b.Fatal(err)
		}
	}
	write("sfdx-project.json", `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
	write("force-app/main/default/objects/Account/Account.object-meta.xml", `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Account</label><pluralLabel>Accounts</pluralLabel><sharingModel>ReadWrite</sharingModel></CustomObject>`)
	write("force-app/main/default/objects/Account/fields/Example__c.field-meta.xml", `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Example__c</fullName><label>Example</label><length>40</length><type>Text</type></CustomField>`)
	bodies := []string{
		`System.assertEquals(2, 1 + 1);`,
		`Account a = new Account(Name='Example'); insert a; System.assertEquals(1, [SELECT count() FROM Account WHERE Id=:a.Id]);`,
		`System.assert(Schema.SObjectType.Account.fields.getMap().containsKey('Name'));`,
		`System.assert(Schema.getGlobalDescribe().containsKey('Account'));`,
	}
	for i := 0; i < 20; i++ {
		name := fmt.Sprintf("ColdOrg%02dTest", i)
		write("force-app/main/default/classes/"+name+".cls", fmt.Sprintf("@IsTest private class %s { @IsTest static void run() { %s } }", name, bodies[i%len(bodies)]))
	}
	p, err := project.Load(root)
	if err != nil {
		b.Fatal(err)
	}
	s, err := schema.LoadProject(p)
	if err != nil {
		b.Fatal(err)
	}
	index := typesys.Build(p, s)
	if len(index.Objects) == 0 {
		b.Fatal("schema overlay missing")
	}
	b.ReportAllocs()
	var orgBuild time.Duration
	b.StopTimer()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		apextest.InvalidateRuntimeCaches()
		b.StartTimer()
		run := apextest.Run(index, apextest.Options{NoDiskCache: true, Parallelism: 1, PerfCounters: true})
		b.StopTimer()
		phases := apextest.SnapshotPerfCounters().Phases
		if phases.CacheMisses != 1 || phases.MemoryCacheHits != 0 || phases.DiskCacheHits != 0 {
			b.Fatalf("iteration %d runtime cache misses=%d memory hits=%d disk hits=%d, want 1/0/0",
				i+1, phases.CacheMisses, phases.MemoryCacheHits, phases.DiskCacheHits)
		}
		orgBuild += time.Duration(phases.OrgBuildNS)
		if summary := run.Summary(); summary.Total != 20 || summary.Passed != 20 {
			b.Fatalf("summary = %#v", summary)
		}
	}
	b.ReportMetric(float64(orgBuild.Nanoseconds())/float64(b.N), "ns/org")
	b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N*20), "ns/test")
}
