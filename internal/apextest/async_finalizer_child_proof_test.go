package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// Exact API63 contract accepted by Salesforce in Wave169: Queueable -> Finalizer -> Queueable.
func TestRunQueueableFinalizerChildAPI63Contract(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"63.0"}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeQFinalizerExactState63.cls"), `
public class GladeQFinalizerExactState63 {
    public static Boolean rootRan = false;
    public static Boolean finalizerRan = false;
    public static Boolean childRan = false;
}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeQFinalizerExactRoot63.cls"), `
public class GladeQFinalizerExactRoot63 implements Queueable {
    public void execute(QueueableContext context) {
        GladeQFinalizerExactState63.rootRan = true;
        insert new Account(Name = 'owned-qfinalizerexact63-root');
        System.attachFinalizer(new GladeQFinalizerExactFinalizer63());
    }
}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeQFinalizerExactFinalizer63.cls"), `
public class GladeQFinalizerExactFinalizer63 implements Finalizer {
    public void execute(FinalizerContext context) {
        GladeQFinalizerExactState63.finalizerRan = true;
        insert new Account(Name = 'owned-qfinalizerexact63-finalizer', Description = String.valueOf(GladeQFinalizerExactState63.rootRan));
        System.enqueueJob(new GladeQFinalizerExactChild63());
    }
}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeQFinalizerExactChild63.cls"), `
public class GladeQFinalizerExactChild63 implements Queueable {
    public void execute(QueueableContext context) {
        GladeQFinalizerExactState63.childRan = true;
        insert new Account(Name = 'owned-qfinalizerexact63-child', Description = String.valueOf(GladeQFinalizerExactState63.finalizerRan));
    }
}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeQFinalizerExactObserve63.cls"), `
@IsTest private class GladeQFinalizerExactObserve63 {
    @IsTest static void queueableFinalizerAndChildAfterStop() {
        Test.startTest();
        Id rootId = System.enqueueJob(new GladeQFinalizerExactRoot63());
        Test.stopTest();
        List<String> effects = new List<String>();
        for (Account row : [SELECT Name, Description FROM Account WHERE Name LIKE 'owned-qfinalizerexact63-%']) effects.add(JSON.serialize(new List<Object>{row.Name, row.Description}));
        effects.sort();
        System.assertEquals(new List<String>{'["owned-qfinalizerexact63-child","true"]','["owned-qfinalizerexact63-finalizer","true"]','["owned-qfinalizerexact63-root",null]'}, effects, 'Exact async effects');
        List<String> jobs = new List<String>();
        for (AsyncApexJob job : [SELECT Id, ApexClass.Name, JobType, Status, ParentJobId, CompletedDate FROM AsyncApexJob WHERE ApexClass.Name IN ('GladeQFinalizerExactRoot63','GladeQFinalizerExactChild63')]) jobs.add(JSON.serialize(new List<Object>{job.ApexClass.Name,job.JobType,job.Status,job.Id == rootId,job.ParentJobId != null,job.ParentJobId == rootId,job.CompletedDate != null}));
        jobs.sort();
        System.assertEquals(new List<String>{'["GladeQFinalizerExactChild63","Queueable","Completed",false,false,false,true]','["GladeQFinalizerExactRoot63","Queueable","Completed",true,false,false,true]'}, jobs, 'Exact Queueable job tuples');
        System.assertNotEquals(null, rootId, 'Root job identity');
        System.assertEquals(true, GladeQFinalizerExactState63.rootRan, 'Caller static rootRan');
        System.assertEquals(true, GladeQFinalizerExactState63.finalizerRan, 'Caller static finalizerRan');
        System.assertEquals(true, GladeQFinalizerExactState63.childRan, 'Caller static childRan');
    }
}`)
	for _, name := range []string{"GladeQFinalizerExactState63", "GladeQFinalizerExactRoot63", "GladeQFinalizerExactFinalizer63", "GladeQFinalizerExactChild63", "GladeQFinalizerExactObserve63"} {
		writeFile(t, filepath.Join(root, "force-app/main/default/classes", name+".cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>63.0</apiVersion><status>Active</status></ApexClass>`)
	}
	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 || got.Failed != 0 || got.Errors != 0 {
		data, _ := json.Marshal(run)
		t.Fatalf("API63 Queueable-finalizer-child contract failed: %s", data)
	}
}
