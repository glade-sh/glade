package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// stagingOrg returns an org whose Account object has an active flow that fails
// the save for a record named Boom after the engine has opened its per-row
// rollback point, plus a populated Contact object no request here touches.
func stagingOrg(t testing.TB, accounts int) storage.OrgState {
	t.Helper()
	org := testOrg()
	account := org.Objects["Account"]
	account.Definition.FlowRules = []storage.FlowRule{{
		Name:     "FailOnBoom",
		Active:   true,
		Criteria: []storage.WorkflowCriteriaItem{{Field: "Name", Operation: "equals", Value: "Boom"}},
		Steps:    []storage.FlowStep{{Kind: "splitLog"}},
	}}
	org.Objects["Account"] = account
	org.Objects["Contact"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Contact",
			KeyPrefix: "003",
			Fields:    map[string]storage.Field{"LastName": {APIName: "LastName", Type: storage.FieldString}},
		},
		Records: map[storage.ID]storage.Record{},
	}
	for i := 0; i < accounts; i++ {
		addAccountForTest(&org, storage.ID(fmt.Sprintf("001%012d", i+1)), fmt.Sprintf("Existing %d", i))
	}
	for i := 0; i < 3; i++ {
		id := storage.ID(fmt.Sprintf("003%012d", i+1))
		org.Objects["Contact"].Records[id] = storage.Record{ID: id, Object: "Contact", Fields: map[string]storage.Value{"LastName": storage.StringValue(fmt.Sprintf("C%d", i))}}
	}
	return org
}

func stagingPost(handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(method, serverTestDataPath+path, strings.NewReader(body)))
	return rec
}

func stagingInsertBody(allOrNone bool, names ...string) string {
	rows := make([]string, len(names))
	for i, name := range names {
		rows[i] = fmt.Sprintf(`{"attributes":{"type":"Account"},"Name":%q}`, name)
	}
	return fmt.Sprintf(`{"allOrNone":%t,"records":[%s]}`, allOrNone, strings.Join(rows, ","))
}

func stagingRows(t *testing.T, rec *httptest.ResponseRecorder) []map[string]any {
	t.Helper()
	var rows []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &rows); err != nil {
		t.Fatalf("decode %s: %v", rec.Body.String(), err)
	}
	return rows
}

func stagingAccountNames(org *storage.OrgState) map[string]int {
	names := map[string]int{}
	for _, record := range org.Objects["Account"].Records {
		names[record.Fields["Name"].String]++
	}
	return names
}

func TestServerDMLPartialSuccessKeepsGoodRowsAndDropsFailedRow(t *testing.T) {
	org := stagingOrg(t, 5)
	before := org.Clone()
	handler := New(&org)

	rec := stagingPost(handler, http.MethodPost, "/composite/sobjects", stagingInsertBody(false, "Good A", "Boom", "Good B", "Boom", "Good C"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	rows := stagingRows(t, rec)
	want := []bool{true, false, true, false, true}
	ids := map[string]bool{}
	for i, row := range rows {
		if row["success"] != want[i] {
			t.Fatalf("row %d = %#v", i, row)
		}
		if id, _ := row["id"].(string); id != "" {
			if ids[id] {
				t.Fatalf("duplicate id %s in %s", id, rec.Body.String())
			}
			ids[id] = true
		}
	}
	names := stagingAccountNames(&org)
	if len(org.Objects["Account"].Records) != 8 || names["Boom"] != 0 || names["Good A"] != 1 || names["Good B"] != 1 || names["Good C"] != 1 {
		t.Fatalf("accounts after partial success = %#v", names)
	}
	for id := range ids {
		if _, ok := org.Objects["Account"].Records[storage.ID(id)]; !ok {
			t.Fatalf("committed id %s missing", id)
		}
	}
	// Failed rows must not burn ids: the next insert continues the sequence.
	next := stagingRows(t, stagingPost(handler, http.MethodPost, "/composite/sobjects", stagingInsertBody(false, "Good D")))
	if next[0]["success"] != true || ids[next[0]["id"].(string)] {
		t.Fatalf("follow-up insert = %#v", next)
	}
	if !reflect.DeepEqual(before.Objects["Contact"].Records, org.Objects["Contact"].Records) {
		t.Fatalf("untouched object changed")
	}
}

// A failed row leaves the id sequence where it found it, so the committed rows
// get the ids a failure-free request would have given them.
func TestServerDMLFailedRowsDoNotBurnIDs(t *testing.T) {
	committed := func(names ...string) []string {
		org := stagingOrg(t, 2)
		rows := stagingRows(t, stagingPost(New(&org), http.MethodPost, "/composite/sobjects", stagingInsertBody(false, names...)))
		var ids []string
		for _, row := range rows {
			if id, _ := row["id"].(string); id != "" && row["success"] == true {
				ids = append(ids, id)
			}
		}
		return ids
	}
	got := committed("A", "Boom", "B", "Boom", "Boom", "C")
	want := committed("A", "B", "C")
	if len(want) != 3 || !reflect.DeepEqual(got, want) {
		t.Fatalf("ids with failed rows = %v, without = %v", got, want)
	}
}

func TestServerDMLAllOrNoneRollbackLeavesLiveOrgUntouched(t *testing.T) {
	org := stagingOrg(t, 5)
	before := org.Clone()
	handler := New(&org)

	rec := stagingPost(handler, http.MethodPost, "/composite/sobjects", stagingInsertBody(true, "Good A", "Good B", "Boom"))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "ALL_OR_NONE_OPERATION_ROLLED_BACK") {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !sameObjectRecords(before, org) || !reflect.DeepEqual(before.IDSequences, org.IDSequences) {
		t.Fatalf("allOrNone rollback changed the live org: %#v", stagingAccountNames(&org))
	}
}

func TestServerDMLFailedWritesLeaveLiveRecordsUntouched(t *testing.T) {
	org := stagingOrg(t, 5)
	before := org.Clone()
	handler := New(&org)
	target := "001000000000001"

	// Single update that fails inside automation.
	rec := stagingPost(handler, http.MethodPatch, "/sobjects/Account/"+target, `{"Name":"Boom"}`)
	if rec.Code < 400 {
		t.Fatalf("failing update status = %d body=%s", rec.Code, rec.Body.String())
	}
	// Single insert that fails inside automation.
	rec = stagingPost(handler, http.MethodPost, "/sobjects/Account", `{"Name":"Boom"}`)
	if rec.Code < 400 {
		t.Fatalf("failing insert status = %d body=%s", rec.Code, rec.Body.String())
	}
	// allOrNone composite update where the second row fails.
	rec = stagingPost(handler, http.MethodPatch, "/composite/sobjects", `{"allOrNone":true,"records":[
{"attributes":{"type":"Account"},"Id":"001000000000002","Description":"changed"},
{"attributes":{"type":"Account"},"Id":"001000000000003","Name":"Boom"}]}`)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "ALL_OR_NONE_OPERATION_ROLLED_BACK") {
		t.Fatalf("composite update status = %d body=%s", rec.Code, rec.Body.String())
	}
	// Partial composite update: the first row commits, the second does not.
	rec = stagingPost(handler, http.MethodPatch, "/composite/sobjects", `{"allOrNone":false,"records":[
{"attributes":{"type":"Account"},"Id":"001000000000004","Description":"kept"},
{"attributes":{"type":"Account"},"Id":"001000000000005","Name":"Boom"}]}`)
	rows := stagingRows(t, rec)
	if len(rows) != 2 || rows[0]["success"] != true || rows[1]["success"] != false {
		t.Fatalf("partial update rows = %s", rec.Body.String())
	}
	records := org.Objects["Account"].Records
	if records["001000000000004"].Fields["Description"].String != "kept" {
		t.Fatalf("partial update lost the good row: %#v", records["001000000000004"].Fields)
	}
	for _, id := range []storage.ID{"001000000000001", "001000000000002", "001000000000003", "001000000000005"} {
		if !sameRecord(before.Objects["Account"].Records[id], records[id]) {
			t.Fatalf("record %s changed: %#v", id, records[id].Fields)
		}
	}
	if len(records) != 5 {
		t.Fatalf("account count = %d", len(records))
	}
}

func TestServerDMLRoutesCommitAndLeaveEarlierReadsStable(t *testing.T) {
	org := stagingOrg(t, 4)
	handler := New(&org)
	// A reference taken before the write must not see a later write.
	heldRecords := org.Objects["Account"].Records
	heldCount := len(heldRecords)

	type route struct{ method, path, body string }
	for _, step := range []route{
		{http.MethodPost, "/sobjects/Account", `{"Name":"Single"}`},
		{http.MethodPatch, "/sobjects/Account/001000000000001", `{"Description":"patched"}`},
		{http.MethodDelete, "/sobjects/Account/001000000000002", ``},
		{http.MethodPatch, "/sobjects/Account/External_Id__c/ext-1", `{"Name":"Upserted"}`},
		{http.MethodPost, "/composite/sobjects", stagingInsertBody(false, "Coll A", "Coll B")},
		{http.MethodPost, "/composite/tree/Account", `{"records":[{"attributes":{"type":"Account","referenceId":"t1"},"Name":"Tree"}]}`},
		{http.MethodPost, "/composite", `{"allOrNone":true,"compositeRequest":[{"method":"POST","url":"` + serverTestDataPath + `/sobjects/Account","referenceId":"r1","body":{"Name":"Sub"}}]}`},
		{http.MethodPost, "/composite/batch", `{"batchRequests":[{"method":"POST","url":"` + serverTestDataPath + `/sobjects/Account","body":{"Name":"Batch"}}]}`},
	} {
		rec := stagingPost(handler, step.method, step.path, step.body)
		if rec.Code >= 400 {
			t.Fatalf("%s %s = %d %s", step.method, step.path, rec.Code, rec.Body.String())
		}
	}
	names := stagingAccountNames(&org)
	for _, name := range []string{"Single", "Upserted", "Coll A", "Coll B", "Tree", "Sub", "Batch"} {
		if names[name] != 1 {
			t.Fatalf("account %q count = %d in %#v", name, names[name], names)
		}
	}
	if org.Objects["Account"].Records["001000000000001"].Fields["Description"].String != "patched" {
		t.Fatalf("patch did not commit")
	}
	if rec := org.Objects["Account"].Records["001000000000002"]; !rec.System.IsDeleted {
		t.Fatalf("delete did not commit: %#v", rec.System)
	}
	if len(heldRecords) != heldCount || heldRecords["001000000000001"].Fields["Description"].String == "patched" {
		t.Fatalf("a record map held before the writes changed")
	}
}

func TestServerDMLConcurrentRequests(t *testing.T) {
	org := stagingOrg(t, 3)
	handler := New(&org)
	const writers, perWriter = 8, 6
	var wg sync.WaitGroup
	var mu sync.Mutex
	ids := map[string]bool{}
	failures := []string{}
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < perWriter; i++ {
				name := fmt.Sprintf("W%d-%d", w, i)
				rec := stagingPost(handler, http.MethodPost, "/composite/sobjects", stagingInsertBody(false, name, "Boom", name+"b"))
				rows := []map[string]any{}
				_ = json.Unmarshal(rec.Body.Bytes(), &rows)
				mu.Lock()
				if len(rows) != 3 || rows[0]["success"] != true || rows[1]["success"] != false || rows[2]["success"] != true {
					failures = append(failures, rec.Body.String())
				} else {
					for _, row := range []int{0, 2} {
						id := rows[row]["id"].(string)
						if ids[id] {
							failures = append(failures, "duplicate id "+id)
						}
						ids[id] = true
					}
				}
				mu.Unlock()
				read := httptest.NewRecorder()
				handler.ServeHTTP(read, httptest.NewRequest(http.MethodGet, serverTestDataPath+"/query?q=SELECT+Id,Name+FROM+Account", nil))
				if read.Code != http.StatusOK {
					mu.Lock()
					failures = append(failures, "query "+read.Body.String())
					mu.Unlock()
				}
			}
		}(w)
	}
	wg.Wait()
	if len(failures) > 0 {
		t.Fatalf("failures: %v", failures[:1])
	}
	want := 3 + writers*perWriter*2
	if got := len(org.Objects["Account"].Records); got != want || len(ids) != want-3 {
		t.Fatalf("accounts = %d ids = %d want %d", got, len(ids), want)
	}
}

func TestServerDMLCompositeInsertAllocationDoesNotGrowWithTable(t *testing.T) {
	measure := func(existing int) uint64 {
		org := stagingOrg(t, existing)
		handler := New(&org)
		names := make([]string, 100)
		for i := range names {
			names[i] = fmt.Sprintf("Row %d", i)
		}
		body := stagingInsertBody(false, names...)
		var before, after runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&before)
		rec := stagingPost(handler, http.MethodPost, "/composite/sobjects", body)
		runtime.ReadMemStats(&after)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d", rec.Code)
		}
		return after.TotalAlloc - before.TotalAlloc
	}
	small, large := measure(50), measure(2000)
	// The table is 40x larger; an insert may not allocate in proportion to it.
	if large > small*8+(8<<20) {
		t.Fatalf("100-row insert allocated %d bytes at 50 rows and %d bytes at 2000 rows", small, large)
	}
	if large > 150<<20 {
		t.Fatalf("100-row insert into 2000 rows allocated %d bytes", large)
	}
}

func sameObjectRecords(a, b storage.OrgState) bool {
	if len(a.Objects) != len(b.Objects) {
		return false
	}
	for name, object := range a.Objects {
		if len(object.Records) != len(b.Objects[name].Records) {
			return false
		}
		for id, record := range object.Records {
			if !sameRecord(record, b.Objects[name].Records[id]) {
				return false
			}
		}
	}
	return true
}

// sameRecord compares the stored content of two records. An empty map and a
// nil map hold the same content.
func sameRecord(a, b storage.Record) bool {
	nulls := func(r storage.Record) map[string]bool {
		out := map[string]bool{}
		for field, isNull := range r.ExplicitNulls {
			if isNull {
				out[field] = true
			}
		}
		return out
	}
	return a.ID == b.ID && a.Object == b.Object && a.System == b.System &&
		reflect.DeepEqual(a.Fields, b.Fields) && reflect.DeepEqual(nulls(a), nulls(b))
}
