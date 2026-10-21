package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// Preserve the uncapped-dispatch behavior preceding the capped-reset fix.
// Native F11/F12 establish reset rejection for a parent with an explicit cap;
// they do not establish that introducing a first cap must also be rejected.
func TestRunUncappedQueueableIntroducesDepthCap(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/DispatchDepthProbe.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/DispatchDepthProbe.cls"), `
@IsTest private class DispatchDepthProbe {
    static Integer calls=0;
    class Child implements Queueable {
        public void execute(QueueableContext context) {calls++;}
    }
    class Dispatcher implements Queueable {
        public void execute(QueueableContext context) {
            AsyncOptions options=new AsyncOptions();
            options.MaximumQueueableStackDepth=3;
            System.enqueueJob(new Child(),options);
        }
    }
    @IsTest static void ordinaryDispatcher() {
        Test.startTest();System.enqueueJob(new Dispatcher());Test.stopTest();
        System.assertEquals(1,calls);
    }
    @IsTest static void delayedDispatcher() {
        Test.startTest();System.enqueueJob(new Dispatcher(),5);Test.stopTest();
        System.assertEquals(1,calls);
    }
    @IsTest static void cappedResetControl() {
        AsyncOptions options=new AsyncOptions();options.MaximumQueueableStackDepth=3;
        Test.startTest();System.enqueueJob(new Dispatcher(),options);
        String caught='';
        try {Test.stopTest();} catch(AsyncException problem) {caught=problem.getMessage();}
        System.assertEquals('Cannot reset Maximum Queueable Stack Depth in a queueable',caught);
        System.assertEquals(0,calls);
    }
    @IsTest static void directRootControl() {
        AsyncOptions options=new AsyncOptions();options.MaximumQueueableStackDepth=3;
        Test.startTest();System.enqueueJob(new Child(),options);Test.stopTest();
        System.assertEquals(1,calls);
    }
}`)
	run := Run(loadTestIndex(t, root), Options{})
	if summary := run.Summary(); summary.Total != 4 || summary.Passed != 4 || summary.Failed != 0 || summary.Errors != 0 {
		data, _ := json.Marshal(run)
		t.Fatalf("uncapped dispatch regression: %s", data)
	}
}
