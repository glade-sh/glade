package vm

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// matrix.tsv holds every native phase B row of the two-private-master junction
// capture for the test route at API 62.0 and 67.0: junction and parent
// UserRecordAccess rows, select row counts, and the ten USER_MODE mutations
// (insert, scalar update and three reparents, each partial and all-or-none),
// for actors U1/U2/U3 at every sharing stage. U1 owns the first-parent records,
// U2 owns the second-parent records, U3 owns neither; stages share only with
// the counterpart owner, except u3_read_both which gives U3 Read on all four.

type junctionMatrixRow struct {
	api, actor, stage, kind string
	json                    []byte
	line                    int
}

func readJunctionMatrix(t *testing.T) []junctionMatrixRow {
	t.Helper()
	data, err := os.ReadFile("testdata/junction_parent_sharing/matrix.tsv")
	if err != nil {
		t.Fatal(err)
	}
	var rows []junctionMatrixRow
	for i, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		if i == 0 {
			continue
		}
		cells := strings.SplitN(line, "\t", 9)
		if len(cells) != 9 {
			t.Fatalf("matrix.tsv:%d has %d cells", i+1, len(cells))
		}
		rows = append(rows, junctionMatrixRow{api: cells[2], kind: cells[5], actor: cells[6], stage: cells[7], json: []byte(unescapeNativeTSV(cells[8])), line: i + 1})
	}
	if len(rows) != 1008 {
		t.Fatalf("matrix.tsv rows=%d, want 1008", len(rows))
	}
	return rows
}

func unescapeNativeTSV(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case '\\':
				b.WriteByte('\\')
				i++
				continue
			case 't':
				b.WriteByte('\t')
				i++
				continue
			case 'r':
				b.WriteByte('\r')
				i++
				continue
			case 'n':
				b.WriteByte('\n')
				i++
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

var junctionMatrixUsers = map[string]string{"U1": "005000000000011", "U2": "005000000000012", "U3": "005000000000013"}

// junctionMatrixMachine builds the junction fixture for one stage and runs as
// actor. Parents are FirstParent__c (owner U1) and SecondParent__c (owner U2),
// two records each; the junction Widget__c links the first record of each.
func junctionMatrixMachine(t *testing.T, api, stage, actor string) *VM {
	t.Helper()
	machine := recordSharingControlMachine(t, "Private")
	machine.currentMethod = Method{APIVersion: api}
	state := machine.Org.Objects["Widget__c"]
	state.Definition.SharingModel = "ControlledByParent"
	state.Records = map[storage.ID]storage.Record{"a00000000000001": {ID: "a00000000000001", Object: "Widget__c", System: storage.SystemFields{OwnerID: "005000000000001"}, Fields: map[string]storage.Value{"Name": storage.StringValue("junction"), "Amount__c": storage.DecimalValue("10"), "First__c": storage.IDValue("a11000000000001"), "Second__c": storage.IDValue("a12000000000001")}}}
	owners := map[string]string{"FirstParent__c": junctionMatrixUsers["U1"], "SecondParent__c": junctionMatrixUsers["U2"]}
	prefixes := map[string]string{"FirstParent__c": "a11", "SecondParent__c": "a12"}
	fields := map[string]string{"FirstParent__c": "First__c", "SecondParent__c": "Second__c"}
	levels := map[string]string{}
	switch stage {
	case "p1_read":
		levels["FirstParent__c"] = "Read"
	case "p2_read":
		levels["SecondParent__c"] = "Read"
	case "both_read":
		levels["FirstParent__c"], levels["SecondParent__c"] = "Read", "Read"
	case "p1_edit_p2_read":
		levels["FirstParent__c"], levels["SecondParent__c"] = "Edit", "Read"
	case "p1_read_p2_edit":
		levels["FirstParent__c"], levels["SecondParent__c"] = "Read", "Edit"
	case "both_edit":
		levels["FirstParent__c"], levels["SecondParent__c"] = "Edit", "Edit"
	}
	for _, parent := range []string{"FirstParent__c", "SecondParent__c"} {
		state.Definition.Fields[fields[parent]] = storage.Field{APIName: fields[parent], Type: storage.FieldReference, MasterDetail: true, ReparentableMasterDetail: true, WriteRequiresMasterRead: true, ReferenceTo: []string{parent}}
		parents := storage.ObjectState{Definition: storage.ObjectDefinition{APIName: parent, SharingModel: "Private", KeyPrefix: prefixes[parent], Fields: map[string]storage.Field{"Id": {APIName: "Id", Type: storage.FieldID}, "Name": {APIName: "Name", Type: storage.FieldString}}}, Records: map[storage.ID]storage.Record{}}
		shares := storage.ObjectState{Definition: storage.ObjectDefinition{APIName: strings.TrimSuffix(parent, "__c") + "__Share"}, Records: map[storage.ID]storage.Record{}}
		for n := 1; n <= 2; n++ {
			id := storage.ID(fmt.Sprintf("%s00000000000%d", prefixes[parent], n))
			parents.Records[id] = storage.Record{ID: id, Object: parent, System: storage.SystemFields{OwnerID: storage.ID(owners[parent])}, Fields: map[string]storage.Value{"Name": storage.StringValue("parent")}}
			grant := func(user, level string) {
				shareID := storage.ID(fmt.Sprintf("a1300000000%04d", len(shares.Records)+1))
				shares.Records[shareID] = storage.Record{ID: shareID, Object: shares.Definition.APIName, Fields: map[string]storage.Value{"ParentId": storage.IDValue(id), "UserOrGroupId": storage.IDValue(storage.ID(user)), "AccessLevel": storage.StringValue(level), "RowCause": storage.StringValue("Manual")}}
			}
			if stage == "u3_read_both" {
				grant(junctionMatrixUsers["U3"], "Read")
			} else if level := levels[parent]; level != "" {
				for _, actorName := range []string{"U1", "U2"} {
					if junctionMatrixUsers[actorName] != owners[parent] {
						grant(junctionMatrixUsers[actorName], level)
					}
				}
			}
		}
		machine.Org.Objects[parent] = parents
		machine.Org.Objects[shares.Definition.APIName] = shares
	}
	machine.Org.Objects["Widget__c"] = state
	objectPermissions := machine.Org.Objects["ObjectPermissions"]
	fieldPermissions := machine.Org.Objects["FieldPermissions"]
	for i, parent := range []string{"FirstParent__c", "SecondParent__c"} {
		id := storage.ID(fmt.Sprintf("110000000000%03d", 50+i))
		objectPermissions.Records[id] = storage.Record{ID: id, Object: "ObjectPermissions", Fields: map[string]storage.Value{"ParentId": storage.IDValue("00e000000000002"), "SObjectType": storage.StringValue(parent), "PermissionsRead": storage.BooleanValue(true), "PermissionsCreate": storage.BooleanValue(true), "PermissionsEdit": storage.BooleanValue(true), "PermissionsDelete": storage.BooleanValue(false)}}
		fid := storage.ID(fmt.Sprintf("0FP0000000001%02d", i))
		fieldPermissions.Records[fid] = storage.Record{ID: fid, Object: "FieldPermissions", Fields: map[string]storage.Value{"ParentId": storage.IDValue("00e000000000002"), "SObjectType": storage.StringValue(parent), "Field": storage.StringValue(parent + ".Name"), "PermissionsRead": storage.BooleanValue(true)}}
	}
	for i, field := range []string{"First__c", "Second__c"} {
		fid := storage.ID(fmt.Sprintf("0FP0000000002%02d", i))
		fieldPermissions.Records[fid] = storage.Record{ID: fid, Object: "FieldPermissions", Fields: map[string]storage.Value{"ParentId": storage.IDValue("00e000000000002"), "SObjectType": storage.StringValue("Widget__c"), "Field": storage.StringValue("Widget__c." + field), "PermissionsRead": storage.BooleanValue(true), "PermissionsEdit": storage.BooleanValue(true)}}
	}
	machine.Org.Objects["ObjectPermissions"] = objectPermissions
	machine.Org.Objects["FieldPermissions"] = fieldPermissions
	users := machine.Org.Objects["User"]
	for _, id := range junctionMatrixUsers {
		users.Records[storage.ID(id)] = storage.Record{ID: storage.ID(id), Object: "User", Fields: map[string]storage.Value{"Id": storage.IDValue(storage.ID(id))}}
	}
	machine.Org.Objects["User"] = users
	user := stripInaccessibleTestUser()
	user.Fields["Id"] = String(junctionMatrixUsers[actor])
	machine.testContext.CurrentUser = user
	machine.SetOrg(machine.Org)
	return machine
}

func runJunctionMatrixApex(t *testing.T, machine *VM, api, source string) error {
	t.Helper()
	program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
	if err != nil {
		t.Fatalf("compile: %v\n%s", err, source)
	}
	_, err = machine.Execute(program)
	return err
}

func TestJunctionControlledParentNativeMatrixAccess(t *testing.T) {
	type key struct{ api, stage, actor string }
	type access struct {
		read, edit, remove, transfer, all bool
		level                             string
	}
	recordAccessTuple := func(r storage.Record) access {
		b := func(name string) bool { return r.Fields[name].Boolean }
		level := r.Fields["MaxAccessLevel"].String
		return access{b("HasReadAccess"), b("HasEditAccess"), b("HasDeleteAccess"), b("HasTransferAccess"), b("HasAllAccess"), level}
	}
	junction := map[key]access{}
	parents := map[key]map[string][]access{}
	selects := map[key]map[string]int{}
	for _, row := range readJunctionMatrix(t) {
		k := key{row.api, row.stage, row.actor}
		switch row.kind {
		case "userRecordAccessRow":
			var native struct {
				Row struct {
					RecordID string `json:"RecordId"`
					Read     bool   `json:"HasReadAccess"`
					Edit     bool   `json:"HasEditAccess"`
					Delete   bool   `json:"HasDeleteAccess"`
					Transfer bool   `json:"HasTransferAccess"`
					All      bool   `json:"HasAllAccess"`
					Level    string `json:"MaxAccessLevel"`
				} `json:"fullRow"`
			}
			if err := json.Unmarshal(row.json, &native); err != nil {
				t.Fatalf("matrix.tsv:%d: %v", row.line, err)
			}
			a := access{native.Row.Read, native.Row.Edit, native.Row.Delete, native.Row.Transfer, native.Row.All, native.Row.Level}
			switch native.Row.RecordID[:3] {
			case "a00":
				junction[k] = a
			default:
				if parents[k] == nil {
					parents[k] = map[string][]access{}
				}
				parents[k][native.Row.RecordID[:3]] = append(parents[k][native.Row.RecordID[:3]], a)
			}
		case "selectSummary":
			var native struct {
				Rows  int    `json:"returnedRows"`
				Probe string `json:"probe"`
			}
			if err := json.Unmarshal(row.json, &native); err != nil {
				t.Fatalf("matrix.tsv:%d: %v", row.line, err)
			}
			if strings.HasSuffix(native.Probe, "WITH_USER_MODE") {
				if selects[k] == nil {
					selects[k] = map[string]int{}
				}
				selects[k][strings.Split(native.Probe, ".")[0]] = native.Rows
			}
		}
	}
	if len(junction) != 48 {
		t.Fatalf("junction access keys=%d, want 48", len(junction))
	}
	var keys []key
	for k := range junction {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return fmt.Sprint(keys[i]) < fmt.Sprint(keys[j])
	})
	for _, k := range keys {
		t.Run(k.api+"/"+k.stage+"/"+k.actor, func(t *testing.T) {
			machine := junctionMatrixMachine(t, k.api, k.stage, k.actor)
			user := junctionMatrixUsers[k.actor]
			stored := machine.Org.Objects["Widget__c"].Records["a00000000000001"]
			want := junction[k]
			if got := machine.userModeRecordVisible("Widget__c", stored, user); got != want.read {
				t.Errorf("junction read=%v native=%v", got, want.read)
			}
			if got := machine.currentUserCanWriteRecord("Widget__c", stored, user, "update"); got != want.edit {
				t.Errorf("junction edit=%v native=%v", got, want.edit)
			}
			if got := recordAccessTuple(machine.userRecordAccessForRecord("Widget__c", stored)); got != want {
				t.Errorf("junction UserRecordAccess=%+v native=%+v", got, want)
			}
			for prefix, object := range map[string]string{"a01": "FirstParent__c", "a02": "SecondParent__c"} {
				var got []access
				var ids []string
				for id := range machine.Org.Objects[object].Records {
					ids = append(ids, string(id))
				}
				sort.Strings(ids)
				for _, id := range ids {
					record := machine.Org.Objects[object].Records[storage.ID(id)]
					got = append(got, recordAccessTuple(machine.userRecordAccessForRecord(object, record)))
				}
				native := parents[k][prefix]
				// Parent URA rows are only recorded for records the actor can read;
				// every recorded row must match a Glade record with the same access,
				// and unlisted records must be unreadable.
				if len(native) != len(got) {
					t.Fatalf("%s native rows=%d, glade records=%d", object, len(native), len(got))
				}
				sortAccess := func(a []access) {
					sort.Slice(a, func(i, j int) bool { return fmt.Sprint(a[i]) < fmt.Sprint(a[j]) })
				}
				sortAccess(got)
				sortAccess(native)
				for i := range got {
					if got[i] != native[i] {
						t.Errorf("%s access=%v native=%v", object, got, native)
						break
					}
				}
			}
			for native, object := range map[string]string{"IspJ__c": "Widget__c", "IspP1__c": "FirstParent__c", "IspP2__c": "SecondParent__c"} {
				if err := runJunctionMatrixApex(t, machine, k.api, fmt.Sprintf("System.assertEquals(%d,[SELECT Id FROM %s WITH USER_MODE].size());", selects[k][native], object)); err != nil {
					t.Errorf("%s USER_MODE select count native %d: %v", object, selects[k][native], err)
				}
			}
		})
	}
}

func TestJunctionControlledParentNativeMatrixMutations(t *testing.T) {
	for _, row := range readJunctionMatrix(t) {
		if row.kind != "mutation" {
			continue
		}
		var native struct {
			Operation   string `json:"operation"`
			AllOrNone   bool   `json:"allOrNone"`
			Outcome     string `json:"outcome"`
			PostInputID string `json:"postInputId"`
			Success     bool   `json:"isSuccess"`
			Errors      []struct {
				Fields  []string `json:"getFields"`
				Status  string   `json:"getStatusCode"`
				Message string   `json:"getMessage"`
			} `json:"getErrors"`
			Exception struct {
				Message string `json:"getMessage"`
				NumDml  int    `json:"getNumDml"`
				Errors  []struct {
					Fields  []string `json:"getDmlFieldNames"`
					Type    string   `json:"getDmlType"`
					Message string   `json:"getDmlMessage"`
					ID      *string  `json:"getDmlId"`
					Index   int      `json:"getDmlIndex"`
				} `json:"errors"`
			} `json:"exception"`
		}
		if err := json.Unmarshal(row.json, &native); err != nil {
			t.Fatalf("matrix.tsv:%d: %v", row.line, err)
		}
		name := fmt.Sprintf("%s/%s/%s/%s/aon=%v", row.api, row.stage, row.actor, native.Operation, native.AllOrNone)
		t.Run(name, func(t *testing.T) {
			machine := junctionMatrixMachine(t, row.api, row.stage, row.actor)
			junctionID := "a00000000000001"
			// glade ids are 15 characters; native 18-character ids truncate to the
			// 15-character form that appears inside native messages.
			fix := func(message string) string {
				if len(native.PostInputID) >= 15 {
					message = strings.ReplaceAll(message, native.PostInputID, "\x00")
					message = strings.ReplaceAll(message, native.PostInputID[:15], junctionID)
				}
				return message
			}
			var statement string
			switch native.Operation {
			case "insert":
				statement = "Widget__c r=new Widget__c(Name='inserted',First__c='a11000000000001',Second__c='a12000000000001');"
			case "scalar":
				statement = "Widget__c r=new Widget__c(Id='" + junctionID + "',Amount__c=20);"
			case "reparent_p1":
				statement = "Widget__c r=new Widget__c(Id='" + junctionID + "',First__c='a11000000000002');"
			case "reparent_p2":
				statement = "Widget__c r=new Widget__c(Id='" + junctionID + "',Second__c='a12000000000002');"
			case "reparent_both":
				statement = "Widget__c r=new Widget__c(Id='" + junctionID + "',First__c='a11000000000002',Second__c='a12000000000002');"
			default:
				t.Fatalf("unknown operation %q", native.Operation)
			}
			dml := "update"
			if native.Operation == "insert" {
				dml = "insert"
			}
			quote := func(s string) string {
				escaped := strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(s)
				return "'" + strings.ReplaceAll(escaped, "\x00", "'+String.valueOf(r.Id)+'") + "'"
			}
			var source strings.Builder
			source.WriteString(statement)
			switch {
			case !native.AllOrNone && native.Outcome == "SaveResult":
				fmt.Fprintf(&source, "Database.SaveResult s=Database.%s(r,false,AccessLevel.USER_MODE);System.assertEquals(%v,s.isSuccess());", dml, native.Success)
				if native.Success {
					source.WriteString("System.assertEquals(0,s.getErrors().size());")
				} else {
					e := native.Errors[0]
					fmt.Fprintf(&source, "System.assertEquals(1,s.getErrors().size());System.assertEquals(%s,String.valueOf(s.getErrors()[0].getStatusCode()));System.assertEquals(%s,s.getErrors()[0].getMessage());System.assertEquals(%d,s.getErrors()[0].getFields().size());System.assertEquals(null,s.getId());", quote(e.Status), quote(fix(e.Message)), len(e.Fields))
				}
			case native.AllOrNone && native.Outcome == "SaveResult":
				fmt.Fprintf(&source, "Database.SaveResult s=Database.%s(r,true,AccessLevel.USER_MODE);System.assertEquals(true,s.isSuccess());System.assertEquals(0,s.getErrors().size());", dml)
			case native.AllOrNone && native.Outcome == "exception":
				e := native.Exception.Errors[0]
				fmt.Fprintf(&source, "Boolean thrown=false;try{Database.%s(r,true,AccessLevel.USER_MODE);}catch(DmlException ex){thrown=true;System.assertEquals(%s,ex.getMessage());System.assertEquals(%d,ex.getNumDml());System.assertEquals(%d,ex.getDmlIndex(0));System.assertEquals(%s,String.valueOf(ex.getDmlType(0)));System.assertEquals(%s,ex.getDmlMessage(0));System.assertEquals(%d,ex.getDmlFieldNames(0).size());", dml, quote(fix(native.Exception.Message)), native.Exception.NumDml, e.Index, quote(e.Type), quote(fix(e.Message)), len(e.Fields))
				if e.ID == nil {
					source.WriteString("System.assertEquals(null,ex.getDmlId(0));")
				} else {
					fmt.Fprintf(&source, "System.assertEquals(%s,String.valueOf(ex.getDmlId(0)).substring(0,15));", quote(junctionID))
				}
				source.WriteString("}System.assert(thrown,'native DmlException');")
			default:
				t.Fatalf("unexpected native outcome %q allOrNone=%v at matrix.tsv:%d", native.Outcome, native.AllOrNone, row.line)
			}
			if err := runJunctionMatrixApex(t, machine, row.api, source.String()); err != nil {
				t.Fatalf("matrix.tsv:%d: %v\n%s", row.line, err, source.String())
			}
		})
	}
}

// TestJunctionControlledParentNativeMatrixWithSharingSelects replays every
// captured "<object>.select.with_sharing" row: SELECT ... WITH SYSTEM_MODE run
// from a with-sharing class, for each actor and sharing stage.
func TestJunctionControlledParentNativeMatrixWithSharingSelects(t *testing.T) {
	objects := map[string]string{"IspJ__c": "Widget__c", "IspP1__c": "FirstParent__c", "IspP2__c": "SecondParent__c"}
	count := 0
	for _, row := range readJunctionMatrix(t) {
		if row.kind != "selectSummary" {
			continue
		}
		var native struct {
			Rows  int    `json:"returnedRows"`
			Probe string `json:"probe"`
		}
		if err := json.Unmarshal(row.json, &native); err != nil {
			t.Fatalf("matrix.tsv:%d: %v", row.line, err)
		}
		if !strings.HasSuffix(native.Probe, ".select.with_sharing") {
			continue
		}
		count++
		object := objects[strings.Split(native.Probe, ".")[0]]
		t.Run(fmt.Sprintf("%s/%s/%s/%s", row.api, row.stage, row.actor, object), func(t *testing.T) {
			machine := junctionMatrixMachine(t, row.api, row.stage, row.actor)
			if err := machine.RegisterClass(Class{Name: "JunctionSharingReader", APIVersion: row.api, Modifiers: []string{"with sharing"}}); err != nil {
				t.Fatal(err)
			}
			source := fmt.Sprintf("System.assertEquals(%d,[SELECT Id FROM %s WITH SYSTEM_MODE].size());", native.Rows, object)
			body, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: row.api})
			if err != nil {
				t.Fatal(err)
			}
			_, err = machine.callMethod(Method{Name: "JunctionSharingReader.read", ClassName: "JunctionSharingReader", IsStatic: true, APIVersion: row.api, SourceContextBound: true, SharingMode: "with sharing", Program: body}, nil, &Result{})
			if err != nil {
				t.Errorf("matrix.tsv:%d native %d rows: %v", row.line, native.Rows, err)
			}
		})
	}
	if count != 144 {
		t.Fatalf("with_sharing selectSummary rows=%d, want 144", count)
	}
}
