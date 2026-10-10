package vm

import (
	"reflect"
	"testing"
)

func TestDetachedCloneIdentityMapAndSetKeys(t *testing.T) {
	for _, zeroRef := range []bool{false, true} {
		name := "withRef"
		if zeroRef {
			name = "withoutRef"
		}
		t.Run(name, func(t *testing.T) {
			machine, _ := runDynamicObjectAliasProgram(t, `
SCWrap first = new SCWrap(); first.Name = 'first';
SCWrap second = new SCWrap(); second.Name = 'second';
Map<SCWrap,String> sourceMap = new Map<SCWrap,String>{first => 'one', second => 'two'};
Set<SCWrap> sourceSet = new Set<SCWrap>{first, second};
`, Class{Name: "SCWrap", Fields: map[string]Field{"Name": {Name: "Name", Type: "String"}}})
			originalMap, originalSet := machine.Globals["sourceMap"], machine.Globals["sourceSet"]
			if zeroRef {
				originalMap.Ref, originalSet.Ref = 0, 0
			}
			graph := cloneValueDetachedPreserveRefs(List(originalMap, originalSet))
			copiedMap, copiedSet := graph.List[0], graph.List[1]
			for i, encoded := range copiedMap.MapOrder {
				key := copiedMap.MapKeys[encoded]
				if encoded != machine.mapKey(key) || key.Ref == originalSet.Set[i].Ref {
					t.Fatalf("key %d has stale encoding or identity: %q Ref=%d", i, encoded, key.Ref)
				}
				if key.Ref != copiedSet.Set[i].Ref || copiedSet.setInsertionHashAt(i) != originalSet.setInsertionHashAt(i) {
					t.Fatal("clone lost shared keys or set insertion history")
				}
			}
			machine.Globals["copiedMap"], machine.Globals["copiedSet"] = copiedMap, copiedSet
			executeAliasEscapeProgram(t, machine, `
List<SCWrap> keys = new List<SCWrap>(copiedMap.keySet());
System.assertEquals('first', keys[0].Name);
System.assertEquals('second', keys[1].Name);
Map<SCWrap,String> freshMap = new Map<SCWrap,String>{keys[0] => 'one', keys[1] => 'two'};
Set<SCWrap> freshSet = new Set<SCWrap>{keys[0], keys[1]};
for (SCWrap key : keys) {
    System.assertEquals(true, copiedMap.containsKey(key));
    System.assertEquals(freshMap.containsKey(key), copiedMap.containsKey(key));
    System.assertEquals(freshMap.put(key, 'replacement'), copiedMap.put(key, 'replacement'));
    System.assertEquals(freshMap.size(), copiedMap.size());
    System.assertEquals(true, copiedSet.contains(key));
    System.assertEquals(freshSet.contains(key), copiedSet.contains(key));
    System.assertEquals(false, copiedSet.add(key));
    System.assertEquals(false, freshSet.add(key));
    System.assertEquals(freshSet.size(), copiedSet.size());
    System.assertEquals(freshMap.remove(key), copiedMap.remove(key));
    System.assertEquals(false, copiedMap.containsKey(key));
    System.assertEquals(freshMap.size(), copiedMap.size());
    System.assertEquals(true, copiedSet.remove(key));
    System.assertEquals(true, freshSet.remove(key));
    System.assertEquals(false, copiedSet.contains(key));
    System.assertEquals(freshSet.size(), copiedSet.size());
}
System.assertEquals(0, copiedMap.size());
System.assertEquals(0, copiedSet.size());
for (SCWrap key : keys) {
    System.assertEquals(freshMap.put(key, 'again'), copiedMap.put(key, 'again'));
    System.assertEquals(true, copiedSet.add(key));
    System.assertEquals(true, freshSet.add(key));
    System.assertEquals(freshMap.size(), copiedMap.size());
    System.assertEquals(freshSet.size(), copiedSet.size());
}
System.assertEquals(2, copiedMap.size());
System.assertEquals(2, copiedSet.size());
System.assertEquals(2, sourceMap.size());
System.assertEquals(2, sourceSet.size());
`)
		})
	}
}

func TestDetachedCloneRetainsHashHistoryAndCycles(t *testing.T) {
	key := Object("MutableKey")
	key.Fields["x"] = Int(2) // The retained custom hash was 1 at insertion.
	source := Map()
	for _, encoded := range []string{"object:MutableKey:hash:1", "object:MutableKey:hash:1\x00collision:1", unhashedMapEntryPrefix + "0"} {
		source.Map[encoded], source.MapKeys[encoded] = String(encoded), key
		source.MapOrder = append(source.MapOrder, encoded)
	}
	// A changed SObject key must retain its insertion encoding too.
	record := Object("Account")
	record.Fields["Name"] = String("before")
	encoded := mapKey(record)
	record.Fields["Name"] = String("after")
	source.Map[encoded], source.MapKeys[encoded] = String("record"), record
	source.MapOrder = append(source.MapOrder, encoded)
	source.MapKeys["__glade_case_insensitive_string_keys"] = Bool(true)
	set := appendSetEntry(Set(), key, setInsertionHash{key: "1", tracked: true})
	source.Map[mapKey(String("set"))] = set
	identity := Object("SCWrap")
	identityKey := mapKey(identity)
	source.Map[identityKey], source.MapKeys[identityKey] = identity, identity
	source.MapOrder = append(source.MapOrder, identityKey)
	source.Map[mapKey(String("self"))] = source
	copy := cloneValueDetachedPreserveRefs(source)
	wantOrder := append([]string(nil), source.MapOrder...)
	clonedIdentity := copy.MapKeys[copy.MapOrder[len(copy.MapOrder)-1]]
	wantOrder[len(wantOrder)-1] = mapKey(clonedIdentity)
	if !reflect.DeepEqual(copy.MapOrder, wantOrder) || !reflect.DeepEqual(copy.Map[mapKey(String("self"))].MapOrder, wantOrder) {
		t.Fatal("clone lost insertion order/history through a cycle")
	}
	if !copy.MapKeys["__glade_case_insensitive_string_keys"].Bool {
		t.Fatal("clone lost map metadata")
	}
	if copy.Map[mapKey(clonedIdentity)].Ref != clonedIdentity.Ref || clonedIdentity.Ref == identity.Ref {
		t.Fatal("clone lost shared key/value identity")
	}
	clonedSet := copy.Map[mapKey(String("set"))]
	if !sameSetInsertionHistory(set, clonedSet) || clonedSet.Set[0].Ref == key.Ref {
		t.Fatal("clone changed custom set insertion history or retained source refs")
	}
	clonedSet.setInsertionHashes[0].key = "changed"
	if set.setInsertionHashAt(0).key != "1" {
		t.Fatal("clone shares set hash storage")
	}
}
