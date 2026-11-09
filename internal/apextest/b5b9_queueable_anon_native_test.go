package apextest

import (
	"os"
	"path/filepath"
	"testing"
)

// b5b9_feature_objects/org.tsv has the 62/67 anonymous observations and
// matching named tests for the signature and the selected option boundaries.
func TestB5B9QueueableNativeAnonymousAndProject(t *testing.T) {
	const fixture = "testdata/conformance/standard_describe_profiles/b5b9_feature_objects/force-app/main/default/classes/"
	cases := []struct{ name, body string }{
		{"signature", `System.QueueableDuplicateSignature.Builder b=QueueableDuplicateSignature.builder();System.assertEquals(0,b.getSize());QueueableDuplicateSignature sig=b.addString('owned').addInteger(42).addId('001000000000001AAA').build();System.assertEquals('\'owned\'_42_001000000000001',sig.toString());AsyncOptions o=new AsyncOptions();o.DuplicateSignature=sig;System.enqueueJob(new OwnedQueue(),o);Boolean failed=false;try{System.enqueueJob(new OwnedQueue(),o);}catch(System.DuplicateMessageException ex){failed=true;}System.assert(failed);`},
		{"delay-negative", `AsyncOptions o=new AsyncOptions();o.MinimumQueueableDelayInMinutes=-1;Boolean failed=false;try{System.enqueueJob(new OwnedQueue(),o);}catch(System.InvalidParameterValueException ex){failed=true;}System.assert(failed);`},
		{"delay-boundary", `AsyncOptions o=new AsyncOptions();o.MinimumQueueableDelayInMinutes=10;System.assertNotEquals(null,System.enqueueJob(new OwnedQueue(),o));`},
		{"depth-zero", `AsyncOptions o=new AsyncOptions();o.MaximumQueueableStackDepth=0;Boolean failed=false;try{System.enqueueJob(new OwnedQueue(),o);}catch(System.AsyncException ex){failed=true;}System.assert(failed);`},
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, tc := range cases {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
				writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), `{"edition":"Developer"}`)
				for _, name := range []string{"OwnedQueue.cls", "OwnedQueue.cls-meta.xml"} {
					data, err := os.ReadFile(fixture + name)
					if err != nil {
						t.Fatal(err)
					}
					writeFile(t, filepath.Join(root, "force-app/main/default/classes", name), string(data))
				}
				runStandardFieldEvidenceRoutes(t, root, api, tc.body)
			})
		}
	}
}
