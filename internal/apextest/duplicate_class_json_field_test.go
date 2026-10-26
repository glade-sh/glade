package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// Two copies of a class can sit under a nested project in one package. The
// runtime merges them but keeps a field slot per source unit. JSON.deserialize
// must fill every slot of a field, whichever unit reads it afterwards.
func TestRunJSONDeserializeFillsEverySourceUnitSlotOfDuplicateClass(t *testing.T) {
	root := t.TempDir()
	meta := "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>44.0</apiVersion><status>Active</status></ApexClass>\n"
	base := func(readerName string) string {
		return `public virtual class DupEnvelope {
    public Detail info;
    public class Detail { public Item[] items; public Integer code; public String message; }
    public class Item { public String domain; }
    public String ` + readerName + `() { return info == null ? 'none' : info.message; }
    public static DupEnvelope parse(String body) { return (DupEnvelope) System.JSON.deserialize(body, DupEnvelope.class); }
}
`
	}
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"60.0","packageDirectories":[{"path":"pkg","default":true}]}`)
	writeFile(t, filepath.Join(root, "pkg/group/sfdx-project.json"), `{"sourceApiVersion":"44.0","packageDirectories":[{"path":"app","default":true}]}`)
	for dir, reader := range map[string]string{"app": "describe", "legacy": "Describe"} {
		writeFile(t, filepath.Join(root, "pkg/group", dir, "DupEnvelope.cls"), base(reader))
		writeFile(t, filepath.Join(root, "pkg/group", dir, "DupEnvelope.cls-meta.xml"), meta)
	}
	writeFile(t, filepath.Join(root, "pkg/group/app/DupEnvelopeTest.cls"), `@IsTest
private class DupEnvelopeTest {
    static final String BODY = '{"info":{"items":[{"domain":"d"}],"code":500,"message":"fail"},"extra":"x"}';
    @IsTest static void fieldIsVisibleAfterDeserialize() {
        DupEnvelope parsed = DupEnvelope.parse(BODY);
        System.assertNotEquals(null, parsed.info, 'info populated');
        System.assertEquals('fail', parsed.info.message);
        System.assertEquals(500, parsed.info.code);
        System.assertEquals(1, parsed.info.items.size());
    }
    @IsTest static void methodReadsDeserializedField() {
        System.assertEquals('fail', DupEnvelope.parse(BODY).describe());
    }
    @IsTest static void typedDeserializeThroughSubclass() {
        DupChild child = (DupChild) System.JSON.deserialize(BODY, DupChild.class);
        System.assertNotEquals(null, child.info, 'inherited info populated');
        System.assertEquals('fail', child.describe());
    }
}
`)
	writeFile(t, filepath.Join(root, "pkg/group/app/DupChild.cls"), "public class DupChild extends DupEnvelope { public String label; }\n")
	for _, name := range []string{"DupEnvelopeTest", "DupChild"} {
		writeFile(t, filepath.Join(root, "pkg/group/app", name+".cls-meta.xml"), meta)
	}

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 3 || got.Passed != 3 || got.Failed != 0 || got.Errors != 0 {
		data, _ := json.Marshal(run)
		t.Fatalf("duplicate class JSON field slots: %s", data)
	}
}
