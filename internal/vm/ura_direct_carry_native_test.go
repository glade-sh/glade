package vm

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// uraDirectCarryGaps pins captured ura_direct_carry rows whose Glade value
// still differs from native, keyed by api/class/probe/object/grant/key. Any
// other difference fails, and so does a pinned row that now matches.
var uraDirectCarryGaps = map[string]bool{}

const uraCarryDirect = "SELECT RecordId,HasReadAccess,HasEditAccess,HasDeleteAccess,HasTransferAccess,HasAllAccess,MaxAccessLevel FROM UserRecordAccess WHERE UserId=:uid AND RecordId=:"
const uraCarryRelFields = "UserRecordAccess.HasReadAccess,UserRecordAccess.HasEditAccess,UserRecordAccess.HasDeleteAccess,UserRecordAccess.HasTransferAccess,UserRecordAccess.HasAllAccess,UserRecordAccess.MaxAccessLevel"

// uraCarryVector stores the six fields of the first direct row under key.
func uraCarryVector(key, rows string) string {
	return fmt.Sprintf("{List<UserRecordAccess> q=%s;p.put('%s',q.isEmpty()?null:(Object)new List<Object>{q[0].HasReadAccess,q[0].HasEditAccess,q[0].HasDeleteAccess,q[0].HasTransferAccess,q[0].HasAllAccess,q[0].MaxAccessLevel});}", rows, key)
}

func uraCarryD(key, rec string) string {
	return uraCarryVector(key, "["+uraCarryDirect+rec+"]")
}

func uraCarryDStatic(key, rec, clause string) string {
	return uraCarryVector(key, "["+uraCarryDirect+rec+clause+"]")
}

func uraCarryDyn(key, rec, clause string) string {
	return fmt.Sprintf("{Id dynRid=%s;%s}", rec, uraCarryVector(key, "Database.query('"+uraCarryDirect+"dynRid"+clause+"')"))
}

func uraCarryRel(object, rec, clause, fields string) string {
	return fmt.Sprintf("{Id relRid=%s;Database.query('SELECT %s FROM %s WHERE Id=:relRid%s');}", rec, fields, object, clause)
}

func uraCarryIDQ(object, rec, clause string) string {
	return fmt.Sprintf("{List<SObject> idq=[SELECT Id FROM %s WHERE Id=:%s%s];}", object, rec, clause)
}

func uraCarryRunAs(body string) string { return "System.runAs(r){" + body + "}" }

const (
	uraUser   = " WITH USER_MODE"
	uraSystem = " WITH SYSTEM_MODE"
)

// uraCarryProbe rebuilds one captured test method (project/gen.py) as Apex.
// The restricted user's direct queries run inside System.runAs(r) unless the
// probe says otherwise; uid is the queried user.
func uraCarryProbe(probe, object string) (string, bool) {
	rel := func(clause string) string { return uraCarryRel(object, "other", clause, uraCarryRelFields) }
	d := uraCarryD("d", "other")
	switch probe {
	case "c1", "f2":
		return uraCarryRunAs(d), true
	case "c2":
		return uraCarryRunAs(uraCarryIDQ(object, "other", uraUser) + d), true
	case "c3":
		return uraCarryRunAs(uraCarryIDQ(object, "other", uraSystem) + d), true
	case "c4":
		return uraCarryRunAs(uraCarryIDQ(object, "other2", uraUser) + d + uraCarryD("d2", "other2")), true
	case "c5":
		return uraCarryRunAs(uraCarryIDQ("Account", "accId", uraUser) + d), true
	case "c6":
		return uraCarryRunAs(uraCarryIDQ(object, "other", uraUser) + uraCarryIDQ(object, "other", uraSystem) + d), true
	case "c7":
		return uraCarryRunAs(uraCarryIDQ(object, "other", uraUser) + "Test.startTest();" + d + "Test.stopTest();"), true
	case "c9", "g2":
		return uraCarryRunAs(rel(uraUser) + d), true
	case "c10", "e0", "g1":
		return uraCarryRunAs(rel(uraSystem) + d), true
	case "c11", "f1":
		return uraCarryRunAs(rel("") + d), true
	case "c12":
		return uraCarryRunAs(uraCarryD("d1", "other") + uraCarryIDQ(object, "other", uraUser) + uraCarryD("d2", "other") + uraCarryIDQ(object, "other", uraSystem) + uraCarryD("d3", "other")), true
	case "c13":
		return uraCarryRunAs(uraCarryIDQ(object, "other", "") + d), true
	case "c14":
		return uraCarryRunAs(fmt.Sprintf("{Id q14=other;Database.query('SELECT Id FROM %s WHERE Id=:q14 WITH USER_MODE');}", object) + d), true
	case "c15":
		return uraCarryRunAs(uraCarryIDQ("Account", "accId", uraSystem) + d), true
	case "c16":
		return uraCarryRunAs(uraCarryDyn("du", "other", uraUser) + d), true
	case "c17":
		return uraCarryRunAs(uraCarryDyn("ds", "other", uraSystem) + d), true
	case "e1":
		return uraCarryRunAs(uraCarryRel(object, "other2", uraSystem, uraCarryRelFields) + d + uraCarryD("d2", "other2")), true
	case "e2":
		return uraCarryRunAs(uraCarryRel("Account", "accId", uraSystem, uraCarryRelFields) + d), true
	case "e3":
		return uraCarryRunAs(uraCarryD("d1", "other") + rel(uraSystem) + uraCarryD("d2", "other")), true
	case "e4":
		return uraCarryRunAs(rel(uraSystem) + "Test.startTest();" + d + "Test.stopTest();"), true
	case "e5":
		return uraCarryRunAs(rel(uraSystem) + rel(uraUser) + d), true
	case "e6":
		return uraCarryRunAs(rel(uraUser) + rel(uraSystem) + d), true
	case "e7":
		return uraCarryRunAs(rel(uraSystem) + uraCarryDyn("du", "other", uraUser) + uraCarryDyn("ds", "other", uraSystem) + d), true
	case "e8":
		return uraCarryRunAs(fmt.Sprintf("{List<SObject> q8=[SELECT %s FROM %s WHERE Id=:other];}", uraCarryRelFields, object) + d), true
	case "e9":
		return rel(uraSystem) + uraCarryRunAs(d), true
	case "e10":
		return uraCarryRunAs(rel(uraSystem)) + uraCarryRunAs(d), true
	case "e11":
		return uraCarryRunAs(uraCarryRel(object, "other", uraSystem, "UserRecordAccess.HasReadAccess") + d), true
	case "e12":
		return uraCarryRunAs(uraCarryRel(object, "other", uraSystem, "UserRecordAccess.MaxAccessLevel") + d), true
	case "e13":
		return uraCarryRunAs(rel("") + d), true
	case "g3":
		return uraCarryRunAs(uraCarryRel(object, "other", uraSystem, "UserRecordAccess.HasDeleteAccess,UserRecordAccess.HasTransferAccess") + d), true
	case "g4":
		return uraCarryRunAs(uraCarryRel(object, "other", uraSystem, "UserRecordAccess.HasEditAccess,UserRecordAccess.HasAllAccess") + d), true
	case "staticWITH-USER_MODE":
		return uraCarryRunAs(uraCarryDStatic("d", "other", uraUser)), true
	case "staticWITH-SYSTEM_MODE":
		return uraCarryRunAs(uraCarryDStatic("d", "other", uraSystem)), true
	case "runas":
		return uraCarryRunAs(uraCarryD("other", "other")), true
	case "mode-user":
		return uraCarryRunAs(uraCarryDyn("other", "other", uraUser)), true
	case "mode-system":
		return uraCarryRunAs(uraCarryDyn("other", "other", uraSystem)), true
	case "mode-dyn":
		return uraCarryRunAs(uraCarryDyn("other", "other", "")), true
	// Administrator rows: the administrator queries its own access outside
	// System.runAs.
	case "admin":
		return "uid=UserInfo.getUserId();" + uraCarryD("A_other", "other"), true
	case "admin-mode-user":
		return "uid=UserInfo.getUserId();" + uraCarryDyn("A_other", "other", uraUser), true
	case "admin-mode-system":
		return "uid=UserInfo.getUserId();" + uraCarryDyn("A_other", "other", uraSystem), true
	case "anon":
		return "uid=UserInfo.getUserId();" + uraCarryD("A_other", "other"), true
	}
	return "", false
}

var uraCarryKeys = map[string]bool{"d": true, "d1": true, "d2": true, "d3": true, "du": true, "ds": true, "other": true, "A_other": true}

var uraCarryObjects = map[string]bool{"Asset": true, "Contract": true, "Order": true, "Account": true, "Opportunity": true, "Pricebook2": true}

func TestURADirectCarryNative(t *testing.T) {
	f, err := os.Open("../apextest/testdata/conformance/ura_direct_carry/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	type group struct{ owd, api, class, sharing, probe, object, grant string }
	want := map[group]map[string]any{}
	for _, r := range rows[1:] {
		// owd source_api class sharing probe object grant key observation
		// Private sharing-model rows, the anonymous route and Pricebook2 are
		// replayed for the administrator's own access only; the restricted
		// rows there depend on record shares Glade does not model.
		if !uraCarryKeys[r[7]] || !uraCarryObjects[r[5]] || strings.HasPrefix(r[2], "LC2PV") {
			continue
		}
		if (r[0] != "ReadWrite" || r[2] == "anonymous" || r[5] == "Pricebook2") && r[7] != "A_other" {
			continue
		}
		if _, ok := uraCarryProbe(r[4], r[5]); !ok {
			continue
		}
		var value any
		if err := json.Unmarshal([]byte(r[8]), &value); err != nil {
			t.Fatalf("bad observation %q: %v", r[8], err)
		}
		if _, ok := value.([]any); !ok {
			continue // "absent" or an error string
		}
		g := group{r[0], r[1], r[2], r[3], r[4], r[5], r[6]}
		if want[g] == nil {
			want[g] = map[string]any{}
		}
		want[g][r[7]] = value
	}
	if len(want) < 500 {
		t.Fatalf("only %d captured probe groups selected", len(want))
	}
	groups := make([]group, 0, len(want))
	for g := range want {
		groups = append(groups, g)
	}
	sort.Slice(groups, func(i, j int) bool {
		return fmt.Sprint(groups[i]) < fmt.Sprint(groups[j])
	})
	seenGap := map[string]bool{}
	for _, g := range groups {
		g := g
		t.Run(strings.Join([]string{g.owd, g.api, g.class, g.probe, g.object, g.grant}, "/"), func(t *testing.T) {
			got := uraCarryRun(t, g.owd, g.api, g.class, g.sharing, g.probe, g.object, g.grant)
			for key, native := range want[g] {
				gap := strings.Join([]string{g.owd, g.api, g.class, g.probe, g.object, g.grant, key}, "/")
				match := reflect.DeepEqual(got[key], native)
				if uraDirectCarryGaps[gap] {
					seenGap[gap] = true
					if match {
						t.Errorf("pinned gap %s now matches native %v: remove it", gap, native)
					}
					continue
				}
				if !match {
					t.Errorf("%s: native %v, glade %v", key, native, got[key])
				}
			}
		})
	}
	for gap := range uraDirectCarryGaps {
		if !seenGap[gap] {
			t.Errorf("pinned gap %s has no captured row", gap)
		}
	}
}

func uraCarryRun(t *testing.T, owd, api, class, sharing, probe, object, grant string) map[string]any {
	t.Helper()
	body, _ := uraCarryProbe(probe, object)
	org := testDataOrg()
	storage.EnsureDeterministicPlatformData(&org)
	storage.EnsureStandardObject(&org, "Account")
	storage.EnsureStandardObject(&org, object)
	org.APIVersion = api
	var profile storage.ID
	for id, p := range org.Objects["Profile"].Records {
		if p.Fields["Name"].String == "Minimum Access - Salesforce" {
			profile = id
			break
		}
	}
	const actor storage.ID = "005000000000098"
	const perm storage.ID = "0PS000000000098"
	objectPerms := []storage.FixtureRecord{{Fields: map[string]storage.Value{"ParentId": storage.IDValue(perm), "SObjectType": storage.StringValue(object), "PermissionsRead": storage.BooleanValue(true), "PermissionsCreate": storage.BooleanValue(true), "PermissionsEdit": storage.BooleanValue(true), "PermissionsDelete": storage.BooleanValue(grant == "delete")}}}
	if object != "Account" {
		objectPerms = append(objectPerms, storage.FixtureRecord{Fields: map[string]storage.Value{"ParentId": storage.IDValue(perm), "SObjectType": storage.StringValue("Account"), "PermissionsRead": storage.BooleanValue(true)}})
	}
	fixture := storage.NewFixture()
	fixture.Objects = []storage.FixtureObject{
		{Name: "User", Records: []storage.FixtureRecord{{ID: actor, Fields: map[string]storage.Value{"Username": storage.StringValue("carry@example.invalid"), "ProfileId": storage.IDValue(profile), "LastName": storage.StringValue("Carry")}}}},
		{Name: "PermissionSet", Records: []storage.FixtureRecord{{ID: perm, Fields: map[string]storage.Value{"Name": storage.StringValue("Carry"), "Label": storage.StringValue("Carry")}}}},
		{Name: "ObjectPermissions", Records: objectPerms},
		{Name: "PermissionSetAssignment", Records: []storage.FixtureRecord{{Fields: map[string]storage.Value{"AssigneeId": storage.IDValue(actor), "PermissionSetId": storage.IDValue(perm)}}}},
	}
	if err := storage.ApplyFixture(&org, fixture); err != nil {
		t.Fatal(err)
	}
	admin := storage.ID(storage.DefaultUserID)
	put := func(name string, ids ...storage.ID) {
		obj := org.Objects[name]
		obj.Definition.SharingModel = owd
		if obj.Records == nil {
			obj.Records = map[storage.ID]storage.Record{}
		}
		for _, id := range ids {
			owner := admin
			if name == "Pricebook2" {
				owner = ""
			}
			obj.Records[id] = storage.Record{ID: id, Object: name, Fields: map[string]storage.Value{}, System: storage.SystemFields{OwnerID: owner}}
		}
		org.Objects[name] = obj
	}
	prefix := org.Objects[object].Definition.KeyPrefix
	other, other2 := storage.ID(prefix+"000000000001"), storage.ID(prefix+"000000000002")
	accID := storage.ID(org.Objects["Account"].Definition.KeyPrefix + "000000000009")
	put("Account", accID)
	put(object, other, other2)
	machine := New(nil)
	machine.SetOrg(&org)
	machine.SetCurrentUser(org.Objects["User"].Records[admin])
	anonymous := class == "anonymous"
	if !anonymous {
		machine.EnableTestContext()
	}
	modifiers := []string{"isTest"}
	if sharing != "none" {
		modifiers = append(modifiers, sharing+" sharing")
	}
	if _, ok := machine.lookupClass(class); !ok && !anonymous {
		if err := machine.RegisterClass(Class{Name: class, APIVersion: api, Modifiers: modifiers}); err != nil {
			t.Fatal(err)
		}
	}
	src := fmt.Sprintf("Map<String,Object> p=new Map<String,Object>();Id uid='%s';User r=new User(Id=uid);Id other='%s';Id other2='%s';Id accId='%s';%sSystem.assert(false,'CARRY'+JSON.serialize(p));", actor, other, other2, accID, body)
	prog, err := CompileAnonymousWithOptions(src, CompileOptions{APIVersion: api})
	if err != nil {
		t.Fatalf("compile %s: %v", probe, err)
	}
	if anonymous {
		_, err = machine.Execute(prog)
	} else {
		_, err = machine.ExecuteTestInClass(prog, class)
	}
	if err == nil {
		t.Fatalf("probe %s did not reach its transport assertion", probe)
	}
	message := err.Error()
	start := strings.Index(message, "CARRY{")
	end := strings.LastIndex(message, "}")
	if start < 0 || end < start {
		t.Fatalf("probe %s failed before transport: %v", probe, err)
	}
	got := map[string]any{}
	if err := json.Unmarshal([]byte(message[start+len("CARRY"):end+1]), &got); err != nil {
		t.Fatalf("probe %s transport %q: %v", probe, message, err)
	}
	return got
}
