package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sobject"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// Native capture: testdata/lightning_action_async_drain/org.tsv. Async work
// enqueued by an @AuraEnabled action (queueable, future, platform event)
// runs after the action's transaction commits, sees the action's DML, never
// runs when the action throws or rolls the enqueue back, and never changes
// what the caller sees.

const asyncDrainController = `public with sharing class AsyncCtrl {
    public class Q implements Queueable {
        String tag; Boolean fail; Boolean chain;
        public Q(String t, Boolean f, Boolean c) { tag = t; fail = f; chain = c; }
        public void execute(QueueableContext qc) {
            Integer seen = [SELECT count() FROM Account WHERE Name = :('D-' + tag)];
            insert new Account(Name = 'Q-' + tag, Description = 'seesAction=' + seen);
            if (chain) { System.enqueueJob(new Q(tag + 'c', false, false)); }
            if (fail) { throw new IllegalArgumentException('queueable-fail'); }
        }
    }
    @future
    public static void fut(String tag) {
        Integer seen = [SELECT count() FROM Account WHERE Name = :('D-' + tag)];
        insert new Account(Name = 'F-' + tag, Description = 'seesAction=' + seen);
    }
    static void act(String tag) { insert new Account(Name = 'D-' + tag, Description = 'action'); }
    @AuraEnabled public static String enqQueueable(String tag) { act(tag); System.enqueueJob(new Q(tag, false, false)); return 'ret'; }
    @AuraEnabled public static String enqChain(String tag) { act(tag); System.enqueueJob(new Q(tag, false, true)); return 'ret'; }
    @AuraEnabled public static String enqQueueableFail(String tag) { act(tag); System.enqueueJob(new Q(tag, true, false)); return 'ret'; }
    @AuraEnabled public static String enqFuture(String tag) { act(tag); fut(tag); return 'ret'; }
    @AuraEnabled public static String pubEvent(String tag) { act(tag); EventBus.publish(new AsyncEvt__e(Tag__c = tag)); return 'ret'; }
    @AuraEnabled public static String allAsync(String tag) { act(tag); System.enqueueJob(new Q(tag, false, false)); fut(tag); EventBus.publish(new AsyncEvt__e(Tag__c = tag)); return 'ret'; }
    @AuraEnabled public static String allThenAura(String tag) { allAsync(tag); throw new AuraHandledException('boom'); }
    @AuraEnabled public static String allThenPlain(String tag) { allAsync(tag); throw new IllegalArgumentException('boom'); }
    @AuraEnabled public static String allThenCatch(String tag) { try { allAsync(tag); throw new IllegalArgumentException('boom'); } catch (Exception e) { return 'caught'; } }
    @AuraEnabled public static String enqSavepoint(String tag) { act(tag); Savepoint sp = Database.setSavepoint(); System.enqueueJob(new Q(tag, false, false)); Database.rollback(sp); return 'ret'; }
    @AuraEnabled public static String enqMany(String tag, Integer n) { act(tag); for (Integer i = 0; i < n; i++) { System.enqueueJob(new Q(tag + '-' + i, false, false)); } return 'ret'; }
    @AuraEnabled public static String futMany(String tag, Integer n) { act(tag); for (Integer i = 0; i < n; i++) { fut(tag + '-' + i); } return 'ret'; }
    @AuraEnabled(cacheable=true) public static String enqCacheable(String tag) { System.enqueueJob(new Q(tag, false, false)); return 'ret'; }
    @AuraEnabled public static String noAsync(String tag) { act(tag); return 'ret'; }
}`

const asyncDrainTrigger = `trigger AsyncEvtTrg on AsyncEvt__e (after insert) {
    for (AsyncEvt__e e : Trigger.new) {
        Integer seen = [SELECT count() FROM Account WHERE Name = :('D-' + e.Tag__c)];
        insert new Account(Name = 'E-' + e.Tag__c, Description = 'seesAction=' + seen);
    }
}`

type asyncDrainHost struct {
	handler *Server
	org     *storage.OrgState
	store   *memoryStore
}

func newAsyncDrainHost(t *testing.T, api string) asyncDrainHost {
	t.Helper()
	root := t.TempDir()
	meta := func(kind string) string {
		return fmt.Sprintf(`<%s xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><status>Active</status></%s>`, kind, api, kind)
	}
	writeLightningFixtureFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
	writeLightningFixtureFile(t, filepath.Join(root, "force-app/main/default/classes/AsyncCtrl.cls"), asyncDrainController)
	writeLightningFixtureFile(t, filepath.Join(root, "force-app/main/default/classes/AsyncCtrl.cls-meta.xml"), meta("ApexClass"))
	writeLightningFixtureFile(t, filepath.Join(root, "force-app/main/default/triggers/AsyncEvtTrg.trigger"), asyncDrainTrigger)
	writeLightningFixtureFile(t, filepath.Join(root, "force-app/main/default/triggers/AsyncEvtTrg.trigger-meta.xml"), meta("ApexTrigger"))
	writeLightningFixtureFile(t, filepath.Join(root, "force-app/main/default/objects/AsyncEvt__e/AsyncEvt__e.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><deploymentStatus>Deployed</deploymentStatus><eventType>HighVolume</eventType><label>Async Evt</label><pluralLabel>Async Evt</pluralLabel><publishBehavior>PublishAfterCommit</publishBehavior></CustomObject>`)
	writeLightningFixtureFile(t, filepath.Join(root, "force-app/main/default/objects/AsyncEvt__e/fields/Tag__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Tag__c</fullName><label>Tag</label><length>80</length><required>false</required><type>Text</type></CustomField>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	schema, err := gladeschema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	source, err := NewSourceMetadataFromProject(p)
	if err != nil {
		t.Fatal(err)
	}
	org := testOrg()
	org.APIVersion = api
	for name, describe := range sobject.BuildDescribeRegistry(schema).Objects {
		if strings.HasSuffix(name, "__e") {
			org.Objects[name] = storage.ObjectState{Definition: sobject.ToObjectDefinition(describe), Records: map[storage.ID]storage.Record{}}
		}
	}
	store := &memoryStore{}
	handler := NewWithStoreAndSource(&org, store, source)
	handler.LimitMode = vm.LimitModeStrict
	caps, _ := vm.LimitCapsForProfile("default")
	handler.LimitCaps = caps
	handler.SetProjectIndex(typesys.Build(p, schema))
	return asyncDrainHost{handler: handler, org: &org, store: store}
}

func (h asyncDrainHost) call(t *testing.T, route, method string, params map[string]any, cacheable bool) lwcbrowser.WireResponse {
	t.Helper()
	var req *http.Request
	switch route {
	case "lightning":
		body, _ := json.Marshal(params)
		req = httptest.NewRequest(http.MethodPost, "/lightning/apex/AsyncCtrl/"+method, strings.NewReader(string(body)))
	case "wire":
		body, _ := json.Marshal(map[string]any{"className": "AsyncCtrl", "method": method, "params": params, "cacheable": cacheable})
		req = httptest.NewRequest(http.MethodPost, "/lightning/wire/apex", strings.NewReader(string(body)))
	default:
		t.Fatalf("route %q", route)
	}
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("%s %s status = %d body = %s", route, method, rec.Code, rec.Body.String())
	}
	var out lwcbrowser.WireResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// accounts lists "Name:Description" for every committed Account.
func (h asyncDrainHost) accounts() []string {
	var rows []string
	for _, record := range h.org.Objects["Account"].Records {
		rows = append(rows, record.Fields["Name"].String+":"+record.Fields["Description"].String)
	}
	sort.Strings(rows)
	return rows
}

func equalRows(got []string, want ...string) bool {
	sort.Strings(want)
	return strings.Join(got, "|") == strings.Join(want, "|")
}

func TestLightningActionAsyncWorkDrainsAfterCommit(t *testing.T) {
	cases := []struct {
		name, method string
		params       map[string]any
		cacheable    bool
		wantData     string
		wantErr      bool
		want         []string
	}{
		{"queueable", "enqQueueable", nil, false, "ret", false, []string{"D-t:action", "Q-t:seesAction=1"}},
		{"chain", "enqChain", nil, false, "ret", false, []string{"D-t:action", "Q-t:seesAction=1", "Q-tc:seesAction=0"}},
		{"queueable throws after commit", "enqQueueableFail", nil, false, "ret", false, []string{"D-t:action"}},
		{"future", "enqFuture", nil, false, "ret", false, []string{"D-t:action", "F-t:seesAction=1"}},
		{"platform event", "pubEvent", nil, false, "ret", false, []string{"D-t:action", "E-t:seesAction=1"}},
		{"all three", "allAsync", nil, false, "ret", false, []string{"D-t:action", "Q-t:seesAction=1", "F-t:seesAction=1", "E-t:seesAction=1"}},
		{"throw AuraHandledException", "allThenAura", nil, false, "", true, nil},
		{"throw plain exception", "allThenPlain", nil, false, "", true, nil},
		{"caught exception commits and drains", "allThenCatch", nil, false, "caught", false, []string{"D-t:action", "Q-t:seesAction=1", "F-t:seesAction=1", "E-t:seesAction=1"}},
		{"savepoint rollback discards the enqueue", "enqSavepoint", nil, false, "ret", false, []string{"D-t:action"}},
		{"cacheable method", "enqCacheable", nil, true, "ret", false, []string{"Q-t:seesAction=0"}},
		{"control without async", "noAsync", nil, false, "ret", false, []string{"D-t:action"}},
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"lightning", "wire"} {
			for _, tc := range cases {
				t.Run(api+"/"+route+"/"+tc.name, func(t *testing.T) {
					host := newAsyncDrainHost(t, api)
					params := map[string]any{"tag": "t"}
					out := host.call(t, route, tc.method, params, tc.cacheable)
					if tc.wantErr {
						if out.Error == nil {
							t.Fatalf("expected the action error, got %#v", out)
						}
					} else if out.Error != nil || out.Data != tc.wantData {
						t.Fatalf("response = %#v error = %+v", out, out.Error)
					}
					if !equalRows(host.accounts(), tc.want...) {
						t.Fatalf("committed accounts = %v, want %v", host.accounts(), tc.want)
					}
				})
			}
		}
	}
}

func TestLightningActionAsyncLimitsFailTheWholeAction(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, method := range []string{"enqMany", "futMany"} {
			t.Run(api+"/"+method, func(t *testing.T) {
				host := newAsyncDrainHost(t, api)
				ok := host.call(t, "lightning", method, map[string]any{"tag": "t", "n": 50}, false)
				if ok.Error != nil || ok.Data != "ret" {
					t.Fatalf("50 jobs = %#v error = %+v", ok, ok.Error)
				}
				if got := len(host.accounts()); got != 51 {
					t.Fatalf("50 async jobs committed %d accounts, want 51", got)
				}
				again := host.call(t, "lightning", method, map[string]any{"tag": "u", "n": 50}, false)
				if again.Error != nil || again.Data != "ret" {
					t.Fatalf("a second action's 50 jobs = %#v error = %+v", again, again.Error)
				}
				fresh := newAsyncDrainHost(t, api)
				over := fresh.call(t, "lightning", method, map[string]any{"tag": "t", "n": 51}, false)
				if over.Error == nil {
					t.Fatalf("51 jobs = %#v", over)
				}
				if got := fresh.accounts(); len(got) != 0 {
					t.Fatalf("51 async jobs committed %v", got)
				}
			})
		}
	}
}
