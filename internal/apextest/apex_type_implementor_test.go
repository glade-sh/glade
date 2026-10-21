package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/testreport"
)

// These API67 metadata shapes were observed in named Apex using owned classes.
func TestRunApexTypeImplementorMetadata(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
	for name, source := range apexTypeImplementorSources {
		writeFile(t, filepath.Join(root, "force-app/main/default/classes", name+".cls"), source)
		writeFile(t, filepath.Join(root, "force-app/main/default/classes", name+".cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>`)
	}
	run := Run(loadTestIndex(t, root), Options{SelectedClasses: []string{"OwnedImplementorTest"}})
	for _, suite := range run.Suites {
		for _, result := range suite.Cases {
			t.Run(result.MethodName, func(t *testing.T) {
				if result.Status != testreport.StatusPass {
					data, _ := json.Marshal(result)
					t.Fatalf("native implementor metadata contract: %s", data)
				}
			})
		}
	}
	if got := run.Summary(); got.Total != 5 || got.Passed != 5 || got.Failed != 0 || got.Errors != 0 || got.Skipped != 0 {
		data, _ := json.Marshal(run)
		t.Fatalf("implementor metadata contracts: %s", data)
	}
}

var apexTypeImplementorSources = map[string]string{
	"AtlasContract": `public interface AtlasContract {
    String name();
}`,
	"AtlasProvider": `public class AtlasProvider implements AtlasContract {
    public String name() { return 'ordinary'; }
}`,
	"AtlasPlain": `public class AtlasPlain {
    public String name() { return 'unrelated'; }
}`,
	"GlyphFixtures": `@IsTest
public class GlyphFixtures {
    public class Matching implements AtlasContract {
        public String name() { return 'public-test-inner'; }
    }
}`,
	"SlateFixtures": `@IsTest
private class SlateFixtures {
    public class Matching implements AtlasContract {
        public String name() { return 'private-test-inner'; }
    }
}`,
	"OwnedImplementorTest": `@IsTest
private class OwnedImplementorTest {
    @IsTest static void publicTestInnerVisible() {
        System.assertNotEquals(null, Type.forName('GlyphFixtures.Matching'));
        List<ApexTypeImplementor> rows = [
            SELECT ClassName, ClassNamespacePrefix, InterfaceName, InterfaceNamespacePrefix, IsConcrete
            FROM ApexTypeImplementor
            WHERE InterfaceName = 'AtlasContract' AND ClassName = 'GlyphFixtures.Matching'
        ];
        System.assertEquals(1, rows.size());
        System.assertEquals('GlyphFixtures.Matching', rows[0].ClassName);
        System.assertEquals(null, rows[0].ClassNamespacePrefix);
        System.assertEquals('AtlasContract', rows[0].InterfaceName);
        System.assertEquals(null, rows[0].InterfaceNamespacePrefix);
        System.assertEquals(true, rows[0].IsConcrete);
    }

    @IsTest static void namespaceFilters() {
        List<ApexTypeImplementor> literalRows = [
            SELECT ClassName
            FROM ApexTypeImplementor
            WHERE ClassName IN ('Matching', 'GlyphFixtures.Matching')
              AND ClassNamespacePrefix IN ('GlyphFixtures', '')
              AND InterfaceName = 'AtlasContract'
            WITH SYSTEM_MODE
        ];
        System.assertEquals(1, literalRows.size());
        System.assertEquals('GlyphFixtures.Matching', literalRows[0].ClassName);
        Set<String> classNames = new Set<String>{'Matching', 'GlyphFixtures.Matching'};
        Set<String> prefixes = new Set<String>{'GlyphFixtures', ''};
        List<ApexTypeImplementor> boundRows = [
            SELECT ClassName
            FROM ApexTypeImplementor
            WHERE ClassName IN :classNames AND ClassNamespacePrefix IN :prefixes
              AND InterfaceName = 'AtlasContract'
            WITH SYSTEM_MODE
        ];
        System.assertEquals(1, boundRows.size());
        System.assertEquals('GlyphFixtures.Matching', boundRows[0].ClassName);
        System.assertEquals(2, [SELECT COUNT() FROM ApexTypeImplementor WHERE InterfaceName = 'AtlasContract' AND ClassNamespacePrefix = null WITH SYSTEM_MODE]);
        System.assertEquals(2, [SELECT COUNT() FROM ApexTypeImplementor WHERE InterfaceName = 'AtlasContract' AND ClassNamespacePrefix = '' WITH SYSTEM_MODE]);
    }

    @IsTest static void topLevelControl() {
        List<ApexTypeImplementor> rows = [
            SELECT ClassName, ClassNamespacePrefix, InterfaceName, InterfaceNamespacePrefix, IsConcrete
            FROM ApexTypeImplementor
            WHERE InterfaceName = 'AtlasContract' AND ClassName = 'AtlasProvider'
        ];
        System.assertEquals(1, rows.size());
        System.assertEquals('AtlasProvider', rows[0].ClassName);
        System.assertEquals(null, rows[0].ClassNamespacePrefix);
        System.assertEquals('AtlasContract', rows[0].InterfaceName);
        System.assertEquals(null, rows[0].InterfaceNamespacePrefix);
        System.assertEquals(true, rows[0].IsConcrete);
    }

    @IsTest static void privateOuterControl() {
        System.assertEquals(0, [
            SELECT COUNT() FROM ApexTypeImplementor
            WHERE InterfaceName = 'AtlasContract' AND ClassName = 'SlateFixtures.Matching'
        ]);
    }

    @IsTest static void unrelatedControl() {
        System.assertEquals(0, [
            SELECT COUNT() FROM ApexTypeImplementor
            WHERE InterfaceName = 'AtlasContract' AND ClassName = 'AtlasPlain'
            WITH SYSTEM_MODE
        ]);
    }
}`,
}

// This local seed-preservation control exercises the org-aware ID allocator;
// it makes no additional Salesforce enumeration claim.
func TestApexTypeImplementorPreservesExistingMetadataRecords(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	for name, source := range map[string]string{
		"AtlasContract": "public interface AtlasContract {}",
		"AtlasProvider": "public class AtlasProvider implements AtlasContract {}",
	} {
		path := filepath.Join(root, "force-app/main/classes", name+".cls")
		writeFile(t, path, source)
		writeFile(t, path+"-meta.xml", `<ApexClass><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>`)
	}
	index := loadTestIndex(t, root)
	org := storage.OrgState{Objects: make(map[string]storage.ObjectState)}
	storage.EnsureStandardObject(&org, "ApexTypeImplementor")
	state := org.Objects["ApexTypeImplementor"]
	ids := storage.NewIDGenerator(map[string]string{"ApexTypeImplementor": state.Definition.KeyPrefix})
	id, err := ids.Next("ApexTypeImplementor")
	if err != nil {
		t.Fatal(err)
	}
	state.Records[id] = storage.Record{ID: id, Object: "ApexTypeImplementor", Fields: map[string]storage.Value{"ClassName": storage.StringValue("SeededProvider")}}
	org.Objects["ApexTypeImplementor"] = state
	applyApexTypeImplementorRecords(&org, index)
	records := org.Objects["ApexTypeImplementor"].Records
	if len(records) != 2 {
		t.Fatalf("metadata rows = %d, want seed and one local implementor", len(records))
	}
	if got := records[id].Fields["ClassName"].String; got != "SeededProvider" {
		t.Fatalf("existing metadata row overwritten: %q", got)
	}
	for recordID, record := range records {
		if recordID == id {
			continue
		}
		if record.Fields["ClassName"].String != "AtlasProvider" {
			t.Fatalf("local row = %#v", record)
		}
		if value, ok := record.Fields["InterfaceNamespacePrefix"]; !ok || value.Kind != storage.ValueNull {
			t.Fatalf("interface namespace = %#v, present=%t; want explicit null", value, ok)
		}
	}
}
