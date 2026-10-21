package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Helper return-type canonicalization must preserve the existing constructor
// contract for a declared record-list view. These namespace cases are local
// restoration controls; native namespace parity is not asserted here.
func TestMapFromDeclaredRecordListRetainsNamespaceIdentity(t *testing.T) {
	for _, api := range []string{"46.0", "62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			org := testDataOrg()
			org.Namespace = "oak"
			for _, name := range []string{"oak__AtlasCard__c", "oak__GlyphCard__c", "fir__GlyphCard__c"} {
				org.Objects[name] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: name, Fields: map[string]storage.Field{
					"Id": {APIName: "Id", Type: storage.FieldID},
				}}}
			}
			machine := New(nil)
			machine.SetOrg(&org)
			body, err := CompileAnonymousWithOptions("return (List<GlyphCard__c>)rows;", CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			for _, class := range []Class{
				{Name: "DeclaredListShelf", Namespace: "oak", APIVersion: api, Methods: map[string]Method{
					"read": {Name: "DeclaredListShelf.read", ClassName: "DeclaredListShelf", APIVersion: api, IsStatic: true, ReturnType: "List<GlyphCard__c>", Params: []Param{{Name: "rows", Type: "List<SObject>"}}, Program: body},
				}},
				{Name: "DeclaredMapProof", Namespace: "oak", APIVersion: api},
			} {
				if err := machine.RegisterClass(class); err != nil {
					t.Fatal(err)
				}
			}
			program, err := CompileAnonymousWithOptions(`
List<SObject> raw=new List<SObject>{new oak__AtlasCard__c(Id='a01000000000001')};
Map<Id,GlyphCard__c> records=new Map<Id,GlyphCard__c>(DeclaredListShelf.read(raw));
System.assertEquals(1,records.size());
System.assertEquals(true,records.containsKey('a01000000000001'));
Map<Id,GlyphCard__c> appended=new Map<Id,GlyphCard__c>();
Boolean putAllRejected=false;
try { appended.putAll(DeclaredListShelf.read(raw)); }
catch(TypeException ex) { putAllRejected=true; }
System.assertEquals(true,putAllRejected);
System.assertEquals(0,appended.size());
Boolean retrievalRejected=false;
try { Object boxed=records.get('a01000000000001'); }
catch(TypeException ex) { retrievalRejected=true; }
System.assertEquals(true,retrievalRejected);
// A generic list does not supply the distinct declared value type.
Boolean rawRejected=false;
try { new Map<Id,GlyphCard__c>(raw); } catch(TypeException ex) { rawRejected=true; }
System.assertEquals(true,rawRejected);
// Equal local names in different packages retain distinct identities.
Boolean otherNamespaceRejected=false;
try { new Map<Id,fir__GlyphCard__c>(DeclaredListShelf.read(raw)); }
catch(TypeException ex) { otherNamespaceRejected=true; }
System.assertEquals(true,otherNamespaceRejected);
Boolean stringKeyRejected=false;
try { new Map<String,GlyphCard__c>(DeclaredListShelf.read(raw)); }
catch(TypeException ex) { stringKeyRejected=true; }
System.assertEquals(true,stringKeyRejected);
List<SObject> mixed=new List<SObject>{new oak__AtlasCard__c(Id='a01000000000001'),new oak__GlyphCard__c(Id='a02000000000001')};
Map<Id,SObject> genericMap=new Map<Id,SObject>(mixed);
System.assertEquals(2,genericMap.size());
System.assertEquals(true,genericMap.containsKey('a01000000000001'));
System.assertEquals(true,genericMap.containsKey('a02000000000001'));
`, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := machine.ExecuteInClass(program, "oak.DeclaredMapProof"); err != nil {
				t.Fatalf("declared-list identity: %v", err)
			}
			for _, name := range []string{"records"} {
				keyed := machine.Globals[name]
				if keyed.Kind != ValueMap || len(keyed.Map) != 1 {
					t.Fatalf("%s missing stored record: %#v", name, keyed)
				}
				for _, value := range keyed.Map {
					if runtimeObjectType(value) != "oak__AtlasCard__c" {
						t.Fatalf("%s changed the stored record's runtime identity: %#v", name, value)
					}
				}
			}
		})
	}
}

// Classes and unknown metadata targets do not acquire record-list trust.
func TestMapRecordListAliasKeepsClassAndUnknownTargets(t *testing.T) {
	for _, target := range []string{"GlyphCard__c", "AbsentCard__c"} {
		t.Run(target, func(t *testing.T) {
			org := testDataOrg()
			org.Namespace = "oak"
			for _, name := range []string{"oak__AtlasCard__c", "oak__GlyphCard__c"} {
				org.Objects[name] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: name}}
			}
			machine := New(nil)
			machine.SetOrg(&org)
			if err := machine.RegisterClass(Class{Name: "GlyphCard__c", Namespace: "oak"}); err != nil {
				t.Fatal(err)
			}
			if err := machine.RegisterClass(Class{Name: "MapScope", Namespace: "oak"}); err != nil {
				t.Fatal(err)
			}
			machine.currentClass = "oak.MapScope"
			record := Object("oak__AtlasCard__c")
			record.Fields["Id"] = platformScalar("Id", "a01000000000001")
			rows := List(record)
			rows.Type = "List<oak__" + target + ">"
			if _, err := machine.mapFromSObjectList("Map<Id,"+target+">", rows); err == nil {
				t.Fatal("class or unknown target acquired metadata alias trust")
			}
		})
	}
}

// Lists without the Map's declared record element retain actual-element
// validation in both constructors and putAll.
func TestMapRecordListAliasKeepsActualElementChecks(t *testing.T) {
	for _, api := range []string{"46.0", "62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			org := testDataOrg()
			org.Namespace = "oak"
			for _, name := range []string{"oak__AtlasCard__c", "oak__GlyphCard__c", "fir__GlyphCard__c"} {
				org.Objects[name] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: name, Fields: map[string]storage.Field{
					"Id": {APIName: "Id", Type: storage.FieldID},
				}}}
			}
			for _, source := range []string{
				"new List<oak__AtlasCard__c>{new oak__AtlasCard__c(Id='a01000000000001')}",
				"new List<SObject>{new oak__AtlasCard__c(Id='a01000000000001')}",
				"new List<SObject>{new oak__GlyphCard__c(Id='a02000000000001'),new oak__AtlasCard__c(Id='a01000000000001')}",
			} {
				machine := New(nil)
				machine.SetOrg(&org)
				if err := machine.RegisterClass(Class{Name: "MapScope", Namespace: "oak", APIVersion: api}); err != nil {
					t.Fatal(err)
				}
				program, err := CompileAnonymousWithOptions(`
Boolean constructorRejected=false;
try { new Map<Id,GlyphCard__c>(`+source+`); }
catch(TypeException ex) { constructorRejected=true; }
System.assertEquals(true,constructorRejected);
Map<Id,GlyphCard__c> appended=new Map<Id,GlyphCard__c>();
Boolean putAllRejected=false;
try { appended.putAll(`+source+`); }
catch(TypeException ex) { putAllRejected=true; }
System.assertEquals(true,putAllRejected);
System.assertEquals(0,appended.size());
List<SObject> matching=new List<SObject>{new oak__GlyphCard__c(Id='a02000000000001')};
Map<Id,GlyphCard__c> accepted=new Map<Id,GlyphCard__c>(matching);
System.assertEquals(1,accepted.size());
System.assertEquals(true,accepted.containsKey('a02000000000001'));
System.assertEquals('a02000000000001',accepted.get('a02000000000001').Id);
Map<Id,GlyphCard__c> matchingAppended=new Map<Id,GlyphCard__c>();
matchingAppended.putAll(matching);
System.assertEquals('a02000000000001',matchingAppended.get('a02000000000001').Id);
`, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := machine.ExecuteInClass(program, "oak.MapScope"); err != nil {
					t.Fatalf("source %s: %v", source, err)
				}
			}
		})
	}
}

// Only installed record payloads may enter the restored alias path. Class
// allocation provenance overrides a record-like spelling and an Id field.
func TestMapRecordListAliasRejectsNonRecordPayload(t *testing.T) {
	org := testDataOrg()
	org.Namespace = "oak"
	for _, name := range []string{"oak__AtlasCard__c", "oak__GlyphCard__c"} {
		org.Objects[name] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: name}}
	}
	for _, tc := range []struct {
		name          string
		runtimeType   string
		classInstance bool
	}{
		{name: "allocated class", runtimeType: "oak.CardEnvelope", classInstance: true},
		{name: "allocated class sharing object spelling", runtimeType: "oak__AtlasCard__c", classInstance: true},
		{name: "unknown nominal record", runtimeType: "oak__AbsentCard__c"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			machine := New(nil)
			machine.SetOrg(&org)
			if err := machine.RegisterClass(Class{Name: "MapScope", Namespace: "oak"}); err != nil {
				t.Fatal(err)
			}
			machine.currentClass = "oak.MapScope"
			payload := Object(tc.runtimeType)
			payload.classInstance = tc.classInstance
			payload.Fields["Id"] = platformScalar("Id", "a01000000000001")
			rows := List(payload)
			rows.Type = "List<oak__GlyphCard__c>"
			if _, err := machine.mapFromSObjectList("Map<Id,GlyphCard__c>", rows); err == nil {
				t.Fatal("metadata alias trusted a non-record payload")
			}
		})
	}
}
