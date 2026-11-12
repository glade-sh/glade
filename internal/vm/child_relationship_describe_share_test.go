package vm

import (
	"fmt"
	"reflect"
	"sync"
	"testing"
)

// Child relationship describe values belong to one runtime schema generation.
// VMs of one primed base share the values until a VM writes one; every outcome
// stays what it was when each VM owned private copies.

func childRelationshipDigest(values []Value) []string {
	out := make([]string, len(values))
	for i, value := range values {
		field, child := value.Fields["field"], value.Fields["childSObject"]
		out[i] = fmt.Sprintf("%s|%s.%s|%s|%v|%v|%v", value.Fields["relationshipName"].Text,
			field.Fields["object"].Text, field.Fields["field"].Text, child.Fields["object"].Text,
			value.Fields["cascadeDelete"].Bool, value.Fields["restrictedDelete"].Bool, len(value.Fields["junctionIdListNames"].List))
	}
	return out
}

func accountChildRelationships(t *testing.T, machine *VM) []Value {
	t.Helper()
	values := machine.describeChildRelationships("Account")
	if len(values) < 3 {
		t.Fatalf("Account has %d child relationships", len(values))
	}
	return values
}

// Fail-before: clones hold the generation's relationship values, not copies.
func TestChildRelationshipDescribeValuesSharedBetweenClones(t *testing.T) {
	for _, namespaced := range []bool{false, true} {
		t.Run(fmt.Sprint(namespaced), func(t *testing.T) {
			base, template := describeSharePrimedBaseFor(t, namespaced)
			first := accountChildRelationships(t, describeShareClone(base, template))
			second := accountChildRelationships(t, describeShareClone(base, template))
			if len(first) != len(second) {
				t.Fatal("clones list different relationships")
			}
			if &first[0] == &second[0] {
				t.Fatal("clones share one list backing: a list write would reach both")
			}
			for i := range first {
				if !sameMapBacking(first[i].Fields, second[i].Fields) {
					t.Fatalf("relationship %d holds private fields", i)
				}
				if !sameMapBacking(first[i].Fields["childSObject"].Fields, second[i].Fields["childSObject"].Fields) {
					t.Fatalf("relationship %d holds a private child token", i)
				}
			}
		})
	}
}

// Shared values read exactly like a private build, through the platform route.
func TestChildRelationshipDescribeEqualsPrivateBuild(t *testing.T) {
	for _, namespaced := range []bool{false, true} {
		t.Run(fmt.Sprint(namespaced), func(t *testing.T) {
			want := childRelationshipDigest(accountChildRelationships(t, describeShareUnprimedFor(namespaced)))
			base, template := describeSharePrimedBaseFor(t, namespaced)
			for range 2 {
				got := childRelationshipDigest(accountChildRelationships(t, describeShareClone(base, template)))
				if !reflect.DeepEqual(got, want) {
					t.Fatal("shared child relationships differ from a private build")
				}
			}
			source := `
List<Schema.ChildRelationship> rels = Schema.SObjectType.Account.getDescribe().getChildRelationships();
System.assert(rels.size() > 2);
Integer withNames = 0;
for (Schema.ChildRelationship rel : rels) {
	System.assertNotEquals(null, rel.getChildSObject());
	System.assertNotEquals(null, rel.getField());
	if (rel.relationshipName != null) { withNames++; }
}
System.assert(withNames > 0);
List<Schema.ChildRelationship> again = Schema.SObjectType.Account.getDescribe().getChildRelationships();
System.assertEquals(rels.size(), again.size());`
			describeShareRunApex(t, describeShareUnprimedFor(namespaced), source)
			describeShareRunApex(t, describeShareClone(base, template), source)
			describeShareRunApex(t, describeShareClone(base, template), source)
		})
	}
}

// Every write route in one VM leaves the shared values and every other VM as
// they were, and the writer reads its own write.
func TestChildRelationshipDescribeSharedValuesSurviveWrites(t *testing.T) {
	const get = `List<Schema.ChildRelationship> rels = Schema.SObjectType.Account.getDescribe().getChildRelationships();
Schema.ChildRelationship rel = rels[1];
`
	routes := []struct{ name, source string }{
		{"relationship field write", get + `rel.relationshipName = 'Changed'; System.assertEquals('Changed', rel.relationshipName); System.assertNotEquals('Changed', Schema.SObjectType.Account.getDescribe().getChildRelationships()[1].relationshipName);`},
		{"compound write", get + `rel.relationshipName += 'X'; System.assert(rel.relationshipName.endsWith('X'));`},
		{"nested token write", get + `rel.childSObject.object = 'ZzContact'; System.assertEquals('ZzContact', String.valueOf(rel.getChildSObject()));`},
		{"token alias write", get + `Schema.SObjectType token = rel.getChildSObject(); token.object = 'ZzContact'; System.assertEquals('ZzContact', String.valueOf(rels[1].getChildSObject()));`},
		{"field token alias write", get + `Schema.SObjectField f = rel.getField(); f.field = 'Changed';`},
		{"list clear", get + `rels.clear(); System.assertEquals(0, rels.size());`},
		{"list remove", get + `Integer n = rels.size(); rels.remove(0); System.assertEquals(n - 1, rels.size());`},
		{"list add", get + `Integer n = rels.size(); rels.add(rel); System.assertEquals(n + 1, rels.size());`},
		{"list set", get + `rels[1] = rels[2]; System.assertEquals(rels[2].relationshipName, rels[1].relationshipName);`},
		{"list add null", get + `rels.add(null); rels[1] = null;`},
		{"list copy write", get + `List<Schema.ChildRelationship> copy = new List<Schema.ChildRelationship>(rels); copy[1].relationshipName = 'Copy'; copy.clear();`},
		{"values loop write", get + `for (Schema.ChildRelationship each : rels) { each.relationshipName = 'Loop'; } System.assertEquals('Loop', rels[2].relationshipName);`},
		{"map holder write", get + `Map<String,Schema.ChildRelationship> byName = new Map<String,Schema.ChildRelationship>{'k' => rel}; byName.get('k').relationshipName = 'Held'; System.assertEquals('Held', rel.relationshipName);`},
		{"junction list write", get + `rel.junctionIdListNames.add('Extra');`},
	}
	for _, namespaced := range []bool{false, true} {
		for _, route := range routes {
			t.Run(fmt.Sprintf("namespace=%v/%s", namespaced, route.name), func(t *testing.T) {
				base, template := describeSharePrimedBaseFor(t, namespaced)
				early := describeShareClone(base, template)
				pristine := childRelationshipDigest(accountChildRelationships(t, early))
				writer := describeShareClone(base, template)
				describeShareRunApex(t, writer, route.source)
				late := describeShareClone(base, template)
				for name, machine := range map[string]*VM{"early": early, "late": late, "unrelated cache hit": describeShareClone(base, template)} {
					if got := childRelationshipDigest(accountChildRelationships(t, machine)); !reflect.DeepEqual(got, pristine) {
						t.Fatalf("%s VM sees changed child relationships", name)
					}
					describeShareRunApex(t, machine, get+`
Integer withNames = 0;
for (Schema.ChildRelationship each : rels) {
	System.assertNotEquals('Changed', each.relationshipName);
	System.assertNotEquals('Loop', each.relationshipName);
	System.assertNotEquals('Copy', each.relationshipName);
	System.assertNotEquals('Held', each.relationshipName);
	System.assertNotEquals('ZzContact', String.valueOf(each.getChildSObject()));
	System.assertEquals(0, each.junctionIdListNames.size());
}
System.assert(rels.size() > 2);`)
				}
			})
		}
	}
}

// A static field that holds relationship values before its VM is cloned keeps
// them when the clone writes through the same static.
func TestChildRelationshipDescribeStaticHoldersSurviveCloneWrites(t *testing.T) {
	mutations := []string{
		`DescribeOwner.rels[1].relationshipName = 'Changed';`,
		`DescribeOwner.rels[1].childSObject.object = 'ZzContact';`,
		`DescribeOwner.rels.clear();`,
		`DescribeOwner.rel.relationshipName = 'Changed';`,
	}
	for _, mutation := range mutations {
		t.Run(mutation, func(t *testing.T) {
			base, template := describeSharePrimedBaseFor(t, false)
			origin := describeShareClone(base, template)
			if err := origin.RegisterClass(Class{Name: "DescribeOwner",
				StaticFields: map[string]Field{
					"rels": {Name: "rels", Type: "List<Schema.ChildRelationship>", Static: true},
					"rel":  {Name: "rel", Type: "Schema.ChildRelationship", Static: true},
				}}); err != nil {
				t.Fatal(err)
			}
			seed, err := CompileAnonymous(`DescribeOwner.rels = Schema.SObjectType.Account.getDescribe().getChildRelationships(); DescribeOwner.rel = DescribeOwner.rels[2];`)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := origin.ExecuteInClass(seed, "DescribeOwner"); err != nil {
				t.Fatal(err)
			}
			pristine := childRelationshipDigest(accountChildRelationships(t, origin))
			clone := origin.CloneRuntime(nil)
			org := template.CloneRuntimeOrg()
			clone.SetRuntimeTemplateOrg(&org)
			clone.EnableTestContext()
			write, err := CompileAnonymous(mutation)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := clone.ExecuteInClass(write, "DescribeOwner"); err != nil {
				t.Fatal(err)
			}
			check, err := CompileAnonymous(`
System.assertNotEquals('Changed', DescribeOwner.rels[1].relationshipName);
System.assertNotEquals('Changed', DescribeOwner.rel.relationshipName);
System.assertNotEquals('ZzContact', String.valueOf(DescribeOwner.rels[1].getChildSObject()));
System.assert(DescribeOwner.rels.size() > 2);`)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := origin.ExecuteInClass(check, "DescribeOwner"); err != nil {
				t.Fatalf("origin VM changed by its clone: %v", err)
			}
			for name, machine := range map[string]*VM{"origin": origin, "later clone": describeShareClone(base, template)} {
				if got := childRelationshipDigest(accountChildRelationships(t, machine)); !reflect.DeepEqual(got, pristine) {
					t.Fatalf("%s sees changed child relationships", name)
				}
			}
		})
	}
}

// Many VMs of one base describe, read and write the shared values at once. Run
// with -race: any in-place write to a shared value is a data race.
func TestChildRelationshipDescribeConcurrentClones(t *testing.T) {
	base, template := describeSharePrimedBaseFor(t, false)
	pristine := childRelationshipDigest(accountChildRelationships(t, describeShareClone(base, template)))
	var group sync.WaitGroup
	errs := make(chan error, 32)
	start := make(chan struct{})
	for worker := 0; worker < 24; worker++ {
		group.Add(1)
		go func(worker int) {
			defer group.Done()
			machine := describeShareClone(base, template)
			const get = `List<Schema.ChildRelationship> rels = Schema.SObjectType.Account.getDescribe().getChildRelationships();`
			var source string
			switch worker % 4 {
			case 0:
				source = get + `System.assert(rels.size() > 2); System.assertNotEquals(null, rels[1].getChildSObject());`
			case 1:
				source = get + `rels[1].relationshipName = 'W'; rels[1].childSObject.object = 'ZzContact'; System.assertEquals('W', rels[1].relationshipName);`
			case 2:
				source = get + `for (Schema.ChildRelationship each : rels) { each.relationshipName = 'L'; } rels.clear();`
			default:
				source = get + `Schema.SObjectType t = rels[1].getChildSObject(); t.object = 'ZzAccount2'; rels.add(rels[1]);`
			}
			program, err := CompileAnonymous(source)
			if err != nil {
				errs <- err
				return
			}
			<-start
			if _, err := machine.Execute(program); err != nil {
				errs <- err
			}
		}(worker)
	}
	close(start)
	group.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	if got := childRelationshipDigest(accountChildRelationships(t, describeShareClone(base, template))); !reflect.DeepEqual(got, pristine) {
		t.Fatal("shared child relationships changed")
	}
}
