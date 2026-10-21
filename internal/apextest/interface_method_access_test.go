package apextest

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/typesys"
)

// The Apex Developer Guide's Access Modifiers section specifies that interface
// methods have the interface's public/global access, without member modifiers.
// https://resources.docs.salesforce.com/latest/latest/en-us/sfdc/pdf/salesforce_apex_developer_guide.pdf
func TestCompileInterfaceMethodInheritsAccess(t *testing.T) {
	for _, access := range []string{"public", "global"} {
		t.Run(access, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace":"cedar","packageDirectories":[{"path":"force-app","default":true}]}`)
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/ToneChannel.cls"), access+` interface ToneChannel {
    String blend(String packet);
    void scatter(Integer packet);
}`)
			index := loadTestIndex(t, root)
			for _, artifact := range []bool{false, true} {
				label := "source"
				if artifact {
					label = "artifact"
					index = typesys.Index{Types: []typesys.TypeSymbol{{
						Name: "ToneChannel", Namespace: "cedar", Kind: apexast.DeclarationInterface,
						Modifiers: []string{access}, Artifact: true, Dependency: true,
						Members: []typesys.MemberSymbol{
							{Name: "blend", Type: "String", Kind: apexast.DeclarationMethod},
							{Name: "scatter", Type: "void", Kind: apexast.DeclarationMethod},
						},
					}}}
				}
				methods := compileProjectMethods(index)
				if len(methods) != 2 {
					t.Fatalf("%s: compiled %d methods, want 2", label, len(methods))
				}
				for _, method := range methods {
					if method.Access != access {
						t.Errorf("%s: %s access = %q, want %q", label, method.Name, method.Access, access)
					}
				}
			}
		})
	}
}

func TestRunCrossNamespaceInterfaceCustomObjectDispatch(t *testing.T) {
	root := t.TempDir()
	provider, subscriber := filepath.Join(root, "provider"), filepath.Join(root, "subscriber")
	for _, project := range []struct{ root, namespace string }{{provider, "cedar"}, {subscriber, "birch"}} {
		writeFile(t, filepath.Join(project.root, "sfdx-project.json"), fmt.Sprintf(`{"namespace":%q,"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`, project.namespace))
	}
	writeFile(t, filepath.Join(subscriber, "glade.yml"), "project:\n  managedPackageDependencies: [\"cedar:../provider\"]\n")
	for _, name := range []string{"Envelope", "Sliver"} {
		writeFile(t, filepath.Join(provider, "force-app/main/default/objects", name+"__c", name+"__c.object-meta.xml"), fmt.Sprintf(`<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>%s</label><pluralLabel>%ss</pluralLabel><nameField><label>Name</label><type>Text</type></nameField><deploymentStatus>Deployed</deploymentStatus><sharingModel>ReadWrite</sharingModel></CustomObject>`, name, name))
	}
	for _, source := range []struct{ root, name, code string }{
		{provider, "ToneChannel", `global interface ToneChannel {
    String blend(Envelope__c packet);
    void scatter(Sliver__c packet);
    String echo(String packet);
}`},
		{provider, "ToneSource", `global class ToneSource implements ToneChannel {
    global String blend(Envelope__c packet) { return 'provider'; }
    global void scatter(Sliver__c packet) {}
    global String echo(String packet) { return 'provider-text'; }
}`},
		{subscriber, "ToneRelay", `public class ToneRelay {
    private cedar.ToneChannel held;
    public ToneRelay(cedar.ToneChannel c) { held = c; }
    private cedar.ToneChannel exposed() { return held; }
    public String relayEnvelope(cedar__Envelope__c packet) { return exposed().blend(packet); }
    public void relaySliver(cedar__Sliver__c packet) { exposed().scatter(packet); }
    public String relayText(String packet) { return exposed().echo(packet); }
}`},
		{subscriber, "ToneChannelProof", `@IsTest private class ToneChannelProof {
    private class QuietTone implements cedar.ToneChannel {
        public String blend(cedar__Envelope__c packet) { return 'subscriber'; }
        public void scatter(cedar__Sliver__c packet) { visits++; }
        public String echo(String packet) { return 'subscriber-text'; }
        public Integer visits = 0;
    }
    @IsTest static void subscriberObjectContractCall() {
        ToneRelay r = new ToneRelay(new QuietTone());
        System.assertEquals('subscriber', r.relayEnvelope(new cedar__Envelope__c()));
    }
    @IsTest static void subscriberObjectVoidCall() {
        QuietTone q = new QuietTone();
        new ToneRelay(q).relaySliver(new cedar__Sliver__c());
        System.assertEquals(1, q.visits);
    }
    @IsTest static void directSubscriberCall() {
        System.assertEquals('subscriber', new QuietTone().blend(new cedar__Envelope__c()));
    }
    @IsTest static void scalarContractCall() {
        System.assertEquals('subscriber-text', new ToneRelay(new QuietTone()).relayText('leaf'));
    }
}`},
	} {
		path := filepath.Join(source.root, "force-app/main/default/classes", source.name+".cls")
		writeFile(t, path, source.code)
		writeFile(t, path+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>`)
	}
	index := loadTestIndex(t, subscriber)
	for _, method := range compileProjectMethods(index) {
		if strings.HasSuffix(method.ClassName, "ToneChannel") && method.Access != "global" {
			t.Errorf("interface method %s access = %q, want global", method.Name, method.Access)
		}
	}
	run := Run(index, Options{})
	if got := run.Summary(); got.Total != 4 || got.Passed != 4 {
		t.Fatalf("summary = %#v; problem = %s", got, firstRunProblem(run))
	}
}
