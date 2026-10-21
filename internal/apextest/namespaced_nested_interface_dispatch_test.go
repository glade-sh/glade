package apextest

import (
	"fmt"
	"path/filepath"
	"testing"
)

// The Apex Developer Guide's Interfaces section requires interface calls to
// execute the implementing class's body. The two receivers deliberately share
// a nested name and field shape; neither changes the other receiver's identity.
// https://resources.docs.salesforce.com/latest/latest/en-us/sfdc/pdf/salesforce_apex_developer_guide.pdf
func TestRunNamespacedNestedInterfaceDispatch(t *testing.T) {
	for _, tc := range []struct {
		name      string
		namespace string
		second    bool
	}{
		{name: "namespaced_two", namespace: "sprig", second: true},
		{name: "namespaced_single", namespace: "sprig"},
		{name: "unnamespaced_two", second: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"namespace":%q,"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`, tc.namespace))
			classes := filepath.Join(root, "force-app/main/default/classes")
			writeFile(t, filepath.Join(classes, "DispatchCradle.cls"), `
public class DispatchCradle {
    public interface Token { String text(); }
    public interface Receiver { void accept(Token item); }
    public class Pebble implements Token {
        public String text() { return 'pebble'; }
    }
    public static Receiver target;
    public static void send() { target.accept(new Pebble()); }
}`)
			names := []string{"DispatchCradleTest"}
			if tc.second {
				names = append(names, "DispatchOtherTest")
			}
			for _, name := range names {
				writeFile(t, filepath.Join(classes, name+".cls"), fmt.Sprintf(`
@IsTest private class %s {
    private class Basket implements DispatchCradle.Receiver {
        public DispatchCradle.Token received;
        public void accept(DispatchCradle.Token item) { received = item; }
    }
    @IsTest static void nestedContract() {
        Basket b = new Basket();
        DispatchCradle.target = b;
        DispatchCradle.send();
        System.assertEquals('pebble', b.received.text());
    }
}`, name))
			}
			for _, name := range append(names, "DispatchCradle") {
				writeFile(t, filepath.Join(classes, name+".cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>`)
			}
			run := Run(loadTestIndex(t, root), Options{})
			if got := run.Summary(); got.Total != len(names) || got.Passed != len(names) {
				t.Fatalf("summary = %#v; problem = %s", got, firstRunProblem(run))
			}
		})
	}
}
