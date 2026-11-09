package apextest

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// event_publish_list_profiles: EventBus.publish inside System.runAs by a user
// without create access to the event. From API 67.0 the publish fails with
// CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY "Access to entity ... denied" for the
// Standard User, Standard Platform User and Minimum Access profiles, for one
// event and for a list; granting create restores success. Below 67.0 it
// succeeds.
func TestEventPublishListProfilesNativeConformance(t *testing.T) {
	runInnerTypeNativeRoutes(t, "event_publish_list_profiles", "PubAccessProbeTest", []string{"isTest"}, func(t *testing.T, got, want any) {
		if diffs := observationDiffs("", got, want); len(diffs) != 0 {
			t.Fatalf("native/Glade differences: %s", strings.Join(diffs, "; "))
		}
	}, "62.0", "66.0", "67.0", "68.0")
}

// automated_process_user_mode: WITH USER_MODE reads and Database USER_MODE
// writes in Automated Process contexts at 62.0, 66.0, 67.0 and 68.0. The isTest
// route observes the event trigger (Automated Process), the platform event flow
// and the test-started queueable and future jobs. Async work enqueued from an
// event trigger does not run inside an Apex test natively, so the job set is
// compared exactly: those jobs must be absent. Their outcome outside a test
// (Automated Process, same USER_MODE results) is recorded in prod.tsv.
func TestAutomatedProcessUserModeNativeConformance(t *testing.T) {
	runInnerTypeNativeRoutes(t, "automated_process_user_mode", "APProbeTest", []string{"isTest"}, func(t *testing.T, got, want any) {
		if diffs := observationDiffs("", got, want); len(diffs) != 0 {
			t.Fatalf("native/Glade differences: %s", strings.Join(diffs, "; "))
		}
	}, "62.0", "66.0", "67.0", "68.0")
}

// observationDiffs lists the paths where Glade's observation differs from the
// native one, with both values.
func observationDiffs(path string, got, want any) []string {
	gotMap, gotIsMap := got.(map[string]any)
	wantMap, wantIsMap := want.(map[string]any)
	if gotIsMap && wantIsMap {
		keys := map[string]bool{}
		for key := range gotMap {
			keys[key] = true
		}
		for key := range wantMap {
			keys[key] = true
		}
		names := make([]string, 0, len(keys))
		for key := range keys {
			names = append(names, key)
		}
		sort.Strings(names)
		var out []string
		for _, key := range names {
			out = append(out, observationDiffs(path+"/"+key, gotMap[key], wantMap[key])...)
		}
		return out
	}
	gotList, gotIsList := got.([]any)
	wantList, wantIsList := want.([]any)
	if gotIsList && wantIsList && len(gotList) == len(wantList) {
		var out []string
		for i := range gotList {
			out = append(out, observationDiffs(fmt.Sprintf("%s[%d]", path, i), gotList[i], wantList[i])...)
		}
		return out
	}
	if reflect.DeepEqual(got, want) {
		return nil
	}
	return []string{fmt.Sprintf("%s native=%v Glade=%v", path, want, got)}
}

// Regression: only async work enqueued inside an event subscriber trigger is
// dropped in a test. A queueable enqueued from an EventBus.publish success
// callback (also run as Automated Process) and one enqueued under System.runAs
// of the Automated Process user still run at Test.stopTest().
func TestAsyncOutsideEventTriggerStillRunsAtStopTest(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			meta := fmt.Sprintf(`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><status>Active</status></ApexClass>`, api)
			classes := "force-app/main/default/classes/"
			writeFile(t, filepath.Join(root, "force-app/main/default/objects/Evt__e/Evt__e.object-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?><CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><deploymentStatus>Deployed</deploymentStatus><eventType>StandardVolume</eventType><label>Evt</label><pluralLabel>Evts</pluralLabel><publishBehavior>PublishAfterCommit</publishBehavior></CustomObject>`)
			for name, body := range map[string]string{
				"CbQ":       `public class CbQ implements Queueable { public void execute(QueueableContext c) { insert new Account(Name = 'ran'); } }`,
				"PubCb":     `public class PubCb implements EventBus.EventPublishSuccessCallback { public void onSuccess(EventBus.SuccessResult r) { System.enqueueJob(new CbQ()); } }`,
				"AsyncTest": `@IsTest private class AsyncTest { @IsTest static void callback() { Test.startTest(); EventBus.publish(new Evt__e(), new PubCb()); Test.stopTest(); System.assertEquals(1, [SELECT COUNT() FROM Account WHERE Name = 'ran']); } @IsTest static void runAsAutomatedProcess() { User ap = [SELECT Id FROM User WHERE UserType = 'AutomatedProcess' LIMIT 1]; Test.startTest(); System.runAs(ap) { System.enqueueJob(new CbQ()); } Test.stopTest(); System.assertEquals(1, [SELECT COUNT() FROM Account WHERE Name = 'ran']); } }`,
			} {
				writeFile(t, filepath.Join(root, classes+name+".cls"), body)
				writeFile(t, filepath.Join(root, classes+name+".cls-meta.xml"), meta)
			}
			result := Run(loadTestIndex(t, root), Options{SelectedClasses: []string{"AsyncTest"}, Parallelism: 1, NoDiskCache: true})
			seen := 0
			for _, suite := range result.Suites {
				for _, c := range suite.Cases {
					seen++
					if c.Problem != nil {
						t.Fatalf("%s: %+v", c.MethodName, c.Problem)
					}
				}
			}
			if seen != 2 {
				t.Fatalf("got %d cases: %+v", seen, result)
			}
		})
	}
}

// automated_process_deliver: with an explicit Test.getEventBus().deliver()
// before Test.stopTest(), the queueable and future jobs enqueued by the event
// trigger run at stopTest (as Automated Process); only stopTest-time delivery
// drops them (automated_process_user_mode).
func TestAutomatedProcessDeliverNativeConformance(t *testing.T) {
	runInnerTypeNativeRoutes(t, "automated_process_deliver", "APProbeTest", []string{"isTest"}, func(t *testing.T, got, want any) {
		if diffs := observationDiffs("", got, want); len(diffs) != 0 {
			t.Fatalf("native/Glade differences: %s", strings.Join(diffs, "; "))
		}
	}, "62.0", "66.0", "67.0", "68.0")
}

// cdc_async_chain: Change Data Capture trigger delivery and async work at
// 62.0, 66.0, 67.0 and 68.0. Each row is one test method of CdcProbeTest. The
// job set is compared exactly, so jobs that natively do not run must be absent
// and a chained enqueue must report the native stack-depth AsyncException.
func TestCDCAsyncChainNativeConformance(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "cdc_async_chain")
	body, err := os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	if len(lines) != 1+4*9 || lines[0] != "source_api\tscenario\tobservation" {
		t.Fatalf("incomplete native matrix: %d lines", len(lines))
	}
	for _, line := range lines[1:] {
		cells := strings.SplitN(line, "\t", 3)
		api, scenario, observation := cells[0], cells[1], cells[2]
		t.Run(api+"/"+scenario, func(t *testing.T) {
			root := t.TempDir()
			project := filepath.Join(dir, "project")
			err := filepath.WalkDir(project, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				content, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				name, err := filepath.Rel(project, path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, name), strings.ReplaceAll(string(content), "62.0", api))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			run := Run(index, Options{SelectedClasses: []string{"CdcProbeTest"}, SelectedMethod: scenario, NoDiskCache: true, Parallelism: 1})
			if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
				t.Fatalf("expected one observation: %+v", run.Summary())
			}
			result := run.Suites[0].Cases[0]
			if result.Problem == nil || !strings.HasSuffix(result.Problem.Type, "AssertException") {
				t.Fatalf("missing assertion transport: %+v; problem: %+v", result, result.Problem)
			}
			_, observed, _ := strings.Cut(result.Problem.Message, "OBS|")
			var got, want any
			if json.Unmarshal([]byte(observed), &got) != nil || json.Unmarshal([]byte(observation), &want) != nil {
				t.Fatalf("invalid observation: %q", observed)
			}
			if diffs := observationDiffs("", got, want); len(diffs) != 0 {
				t.Fatalf("native/Glade differences: %s", strings.Join(diffs, "; "))
			}
		})
	}
}
