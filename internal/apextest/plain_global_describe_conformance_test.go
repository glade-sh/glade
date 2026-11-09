package apextest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

const plainGlobalDescribeFixture = "testdata/conformance/plain_global_describe"

// plainGlobalDescribeCatalogOmissions lists native plain Developer keys the
// generated standard catalog has no describe for. They must stay missing, or
// the list is stale.
var plainGlobalDescribeCatalogOmissions = map[string][]string{
	"62": {"aggregateresult", "analyticstaskstatusevent", "approvalworkitemshare", "asyncoperationevent", "asyncoperationstatus", "curatedexpjobevent", "curatedexpplanevent", "customreporttype", "emailbounceevent", "mlengagementevent", "mobilepushservicedevice", "msgsessstatuschangedevent", "orglifecyclenotification", "packageconversion", "productconsumptionschedule", "quotetemplaterichtextdata", "sourcechangenotification", "workforcecapacityshare", "workloadshare"},
	"67": {"actorreactiveaskrespevent", "aggregateresult", "analyticstaskstatusevent", "approvalworkitemshare", "asyncoperationevent", "asyncoperationstatus", "curatedexpjobevent", "curatedexpplanevent", "customreporttype", "devopsrequestinfoevent", "emailbounceevent", "featureusageevent", "fieldmappingconfig", "fieldmappingconfigitem", "gisprvdcnctrjobobjrun", "mlengagementevent", "mobilepushservicedevice", "msgsessstatuschangedevent", "orglifecyclenotification", "outboundengagementtmplview", "outboundengmtcontentrsrc", "outboundengmtcontentrsrcfeed", "outboundengmtcontentrsrcshare", "packageconversion", "productconsumptionschedule", "quotetemplaterichtextdata", "reporthierarchyobjectfield", "sfdcpartnersbscrofferitemhistory", "sourcechangenotification", "universalanomalyevent", "workforcecapacityshare", "workitemevent", "workloadshare"},
}

// plainGlobalDescribeTestOmissions adds the isTest-only native keys the
// catalog has no describe for.
var plainGlobalDescribeTestOmissions = map[string][]string{
	"62": {"activityrecurrence2", "activityrecurrence2exception", "activityrecurrence2exceptionshare", "activityrecurrence2share", "aidatadefinition", "aierror", "aiinsightreasonmlmodelfactor", "aimodel", "aimodeldefinition", "aimodelgraph", "aipredictiondefinition", "aistate", "backgroundopoutcomeevent", "backgroundopstatusevent", "dataassetusagetrackinginfo", "directmessagefeed", "directmessagemember", "engagementevent", "experiencediagnosticevent", "filebaseddataimport", "filebaseddataimportshare", "mlmigration", "mlpipelineevent", "s2xeventmap", "tenantsecuritymetricdetail", "tenantsecuritypolicychangelog", "tenantsecuritytenantchangelog", "workforecastdemogrun", "workforecastrun", "workforecastrunshare"},
	"67": {"activityrecurrence2", "activityrecurrence2exception", "activityrecurrence2exceptionshare", "activityrecurrence2share", "aidatadefinition", "aierror", "aiinsightreasonmlmodelfactor", "aimodel", "aimodeldefinition", "aimodelgraph", "aipredictiondefinition", "aistate", "backgroundopoutcomeevent", "backgroundopstatusevent", "dataassetusagetrackinginfo", "directmessagefeed", "directmessagemember", "engagementevent", "experiencediagnosticevent", "filebaseddataimport", "filebaseddataimportshare", "flowappvlproccmplevent", "mcptaskprogressevent", "mlmigration", "mlpipelineevent", "s2xeventmap", "tenantscristgnrmdnplan", "tenantscristgnrmdnplanitem", "tenantsecurityanomaly", "tenantsecurityfieldclsfn", "tenantsecurityfinding", "tenantsecurityimpactedinst", "tenantsecurityinvestigation", "tenantsecurityistgnfinding", "tenantsecurityistgntmlnentry", "tenantsecuritymetricdetail", "tenantsecuritypolicychangelog", "tenantsecuritytenantchangelog", "tenantsecuritytmlnentry", "tenantusagetypemultiplierdef", "workforecastdemogrun", "workforecastrun", "workforecastrunshare"},
}

// global_describe_changeevent_full/org.tsv (anonymous, 62.0 to 68.0, a fresh
// Developer org with deployed custom objects) lists this key on the anonymous
// route; plain_global_describe lists it on isTest only. Glade keeps the
// change event table's answer.
const plainGlobalDescribeConflictingKey = "collabtemplatemetricchangeevent"

func plainGlobalDescribeKeys(t *testing.T, name string) map[string]bool {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(plainGlobalDescribeFixture, name))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for _, key := range strings.Fields(string(body)) {
		out[key] = true
	}
	return out
}

const plainGlobalDescribeTestClass = `@IsTest
private class PlainKeysTest {
    @IsTest static void keys() {
        List<String> ks = new List<String>(Schema.getGlobalDescribe().keySet());
        ks.sort();
        System.assert(false, 'OBS|' + JSON.serialize(ks));
    }
}`

func plainGlobalDescribeProject(t *testing.T, api, definition string) typesys.Index {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
	if definition != "" {
		writeFile(t, filepath.Join(root, "config", "project-scratch-def.json"), definition)
	}
	classes := filepath.Join(root, "force-app", "main", "default", "classes")
	writeFile(t, filepath.Join(classes, "PlainKeysTest.cls"), plainGlobalDescribeTestClass)
	writeFile(t, filepath.Join(classes, "PlainKeysTest.cls-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?>
<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
	index := loadTestIndex(t, root)
	if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
		t.Fatalf("project compilation: %v %v", index.Diagnostics, analysis.Diagnostics)
	}
	return index
}

func plainGlobalDescribeAnonymous(t *testing.T, index typesys.Index, api string) map[string]bool {
	t.Helper()
	org := capturedFixtureOrg(t, index, api)
	runner := newConformanceRunner(t, index, conformanceRunnerOptions{Org: &org, LinkProject: true})
	program, err := vm.CompileAnonymousWithOptions("for (String k : Schema.getGlobalDescribe().keySet()) System.debug(LoggingLevel.ERROR, 'KEY|' + k);", vm.CompileOptions{APIVersion: api})
	if err != nil {
		t.Fatal(err)
	}
	result, err := runner.execute(program)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for _, line := range result.Debug {
		if _, key, ok := strings.Cut(line, "KEY|"); ok {
			out[strings.TrimSpace(key)] = true
		}
	}
	return out
}

func plainGlobalDescribeIsTest(t *testing.T, index typesys.Index) map[string]bool {
	t.Helper()
	run := Run(index, Options{SelectedClasses: []string{"PlainKeysTest"}, SelectedMethod: "keys", NoDiskCache: true, Parallelism: 1})
	if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
		t.Fatalf("expected the keys observation: %+v", run.Summary())
	}
	_, body, ok := strings.Cut(run.Suites[0].Cases[0].Problem.Message, "OBS|")
	if !ok {
		t.Fatalf("missing observation: %s", run.Suites[0].Cases[0].Problem.Message)
	}
	var keys []string
	if err := json.Unmarshal([]byte(body), &keys); err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for _, key := range keys {
		out[key] = true
	}
	return out
}

func plainGlobalDescribeCompare(t *testing.T, major string, isTest bool, want, got map[string]bool, allowExtra map[string]bool) {
	t.Helper()
	omitted := map[string]bool{}
	omissions := plainGlobalDescribeCatalogOmissions[major]
	if isTest {
		omissions = append(append([]string(nil), omissions...), plainGlobalDescribeTestOmissions[major]...)
	}
	for _, key := range omissions {
		omitted[key] = true
		if got[key] {
			t.Errorf("catalog omission %s is now listed; remove it from plainGlobalDescribeCatalogOmissions", key)
		}
	}
	var missing, extra []string
	for key := range want {
		if !got[key] && !omitted[key] {
			missing = append(missing, key)
		}
	}
	for key := range got {
		if !want[key] && !allowExtra[key] {
			extra = append(extra, key)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	if len(missing) > 0 || len(extra) > 0 {
		t.Fatalf("native-only %d %v; Glade-only %d %v", len(missing), missing, len(extra), extra)
	}
}

// Native rows: plain_global_describe org.tsv globalKeys (anonymous and isTest,
// 62.0 and 67.0, plain and settings shapes).
func TestPlainGlobalDescribeNativeConformance(t *testing.T) {
	shapes := map[string]string{}
	for _, shape := range []string{"plain", "settings"} {
		body, err := os.ReadFile(filepath.Join(plainGlobalDescribeFixture, "shapes", shape+".json"))
		if err != nil {
			t.Fatal(err)
		}
		shapes[shape] = string(body)
	}
	for _, api := range []string{"62.0", "67.0"} {
		major := strings.TrimSuffix(api, ".0")
		anonymous := plainGlobalDescribeKeys(t, "keys-anonymous-"+major+".txt")
		isTest := plainGlobalDescribeKeys(t, "keys-test-only-"+major+".txt")
		for key := range anonymous {
			isTest[key] = true
		}
		for _, shape := range []string{"plain", "settings"} {
			t.Run(api+"/"+shape, func(t *testing.T) {
				index := plainGlobalDescribeProject(t, api, shapes[shape])
				plainGlobalDescribeCompare(t, major, false, anonymous, plainGlobalDescribeAnonymous(t, index, api), map[string]bool{plainGlobalDescribeConflictingKey: true})
				plainGlobalDescribeCompare(t, major, true, isTest, plainGlobalDescribeIsTest(t, index), nil)
			})
		}
	}
}

// A shape outside the captured plain declarations, or no declaration at all,
// cannot establish absence: declared features keep adding their objects.
func TestPlainGlobalDescribeUncapturedShapesKeepCatalog(t *testing.T) {
	native := plainGlobalDescribeKeys(t, "keys-anonymous-67.txt")
	for name, definition := range map[string]string{
		"unspecified": "",
		"feature":     `{"edition":"Developer","features":["Communities"],"settings":{"communitiesSettings":{"enableNetworksEnabled":true}}}`,
		"setting":     `{"edition":"Developer","settings":{"quoteSettings":{"enableQuote":true}}}`,
		"enterprise":  `{"edition":"Enterprise"}`,
	} {
		t.Run(name, func(t *testing.T) {
			index := plainGlobalDescribeProject(t, "67.0", definition)
			got := plainGlobalDescribeAnonymous(t, index, "67.0")
			extra := 0
			for key := range got {
				if !native[key] {
					extra++
				}
			}
			if extra < 1000 {
				t.Fatalf("%s shape lost its catalog fallback: %d keys beyond the plain list", name, extra)
			}
			if name == "feature" && !got["network"] {
				t.Fatalf("declared Communities feature no longer adds Network")
			}
			if name == "setting" && !got["quote"] {
				t.Fatalf("declared quote setting no longer adds Quote")
			}
		})
	}
}

const plainPrivacyProbe = `Map<String, Object> out = new Map<String, Object>();
Schema.DescribeSObjectResult d = Schema.getGlobalDescribe().get('PrivacyRTBFRequest').getDescribe();
out.put('present', true);
out.put('keyPrefix', d.getKeyPrefix());
out.put('label', d.getLabel());
out.put('flags', 'c=' + d.isCreateable() + ',q=' + d.isQueryable() + ',u=' + d.isUpdateable() + ',d=' + d.isDeletable());
List<String> fields = new List<String>();
for (Schema.SObjectField f : d.fields.getMap().values()) {
    Schema.DescribeFieldResult x = f.getDescribe();
    fields.add(x.getName() + ':' + x.getType() + ':' + x.isRestrictedPicklist() + ':' + x.isCreateable() + ':' + x.isNillable());
}
fields.sort();
out.put('fields', fields);
System.debug(LoggingLevel.ERROR, 'OBS|' + JSON.serialize(out));
for (String s : new List<String>{'Task.WhatId', 'Event.WhatId'}) {
    List<String> p = s.split('\\.');
    Boolean listed = false;
    for (Schema.SObjectType t : Schema.getGlobalDescribe().get(p[0]).getDescribe().fields.getMap().get(p[1]).getDescribe().getReferenceTo()) {
        if (String.valueOf(t) == 'PrivacyRTBFRequest') listed = true;
    }
    System.debug(LoggingLevel.ERROR, 'TARGET|' + s + '|' + listed);
}`

// Native rows: plain_global_describe org.tsv privacyRTBFRequest, Task.WhatId
// and Event.WhatId (62.0 and 67.0). The describe is compared whole; the target
// lists only for PrivacyRTBFRequest (the other Task.WhatId differences belong
// to the record_type_shape plain_targets rows).
func TestPlainPrivacyRTBFRequestNativeConformance(t *testing.T) {
	rows := map[string]map[string]string{}
	body, err := os.ReadFile(filepath.Join(plainGlobalDescribeFixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(body)), "\n")[1:] {
		cells := strings.Split(line, "\t")
		if rows[cells[0]] == nil {
			rows[cells[0]] = map[string]string{}
		}
		rows[cells[0]][cells[3]] = cells[4]
	}
	shape, err := os.ReadFile(filepath.Join(plainGlobalDescribeFixture, "shapes", "plain.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			index := plainGlobalDescribeProject(t, api, string(shape))
			org := capturedFixtureOrg(t, index, api)
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{Org: &org, LinkProject: true})
			program, err := vm.CompileAnonymousWithOptions(plainPrivacyProbe, vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			result, err := runner.execute(program)
			if err != nil {
				t.Fatal(err)
			}
			var got map[string]any
			targets := map[string]bool{}
			for _, line := range result.Debug {
				if _, obs, ok := strings.Cut(line, "OBS|"); ok {
					if err := json.Unmarshal([]byte(obs), &got); err != nil {
						t.Fatal(err)
					}
				}
				if _, target, ok := strings.Cut(line, "TARGET|"); ok {
					name, listed, _ := strings.Cut(target, "|")
					targets[name] = listed == "true"
				}
			}
			var want map[string]any
			if err := json.Unmarshal([]byte(rows[api]["privacyRTBFRequest"]), &want); err != nil {
				t.Fatal(err)
			}
			// record_type_shape (lane N18): RecordTypeId exists only on objects
			// with a record type. Until that fix lands Glade lists it on every
			// object; it is not part of this row's question.
			if fields, ok := got["fields"].([]any); ok {
				kept := fields[:0]
				for _, field := range fields {
					if text, _ := field.(string); !strings.HasPrefix(text, "RecordTypeId:") {
						kept = append(kept, field)
					}
				}
				got["fields"] = kept
			}
			gotJSON, _ := json.Marshal(got)
			wantJSON, _ := json.Marshal(want)
			if string(gotJSON) != string(wantJSON) {
				t.Errorf("describe: native %s; Glade %s", wantJSON, gotJSON)
			}
			for _, field := range []string{"Task.WhatId", "Event.WhatId"} {
				var native []string
				if err := json.Unmarshal([]byte(rows[api][field]), &native); err != nil {
					t.Fatal(err)
				}
				listed := false
				for _, name := range native {
					listed = listed || name == "PrivacyRTBFRequest"
				}
				if !listed || !targets[field] {
					t.Errorf("%s: native lists PrivacyRTBFRequest %v; Glade %v", field, listed, targets[field])
				}
			}
		})
	}
}
