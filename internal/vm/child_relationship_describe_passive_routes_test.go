package vm

import (
	"fmt"
	"reflect"
	"sort"
	"sync"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

const passivePre = `List<Schema.ChildRelationship> rels = Schema.SObjectType.Account.getDescribe().getChildRelationships(); Schema.ChildRelationship cr = rels[1]; Schema.SObjectField fld = cr.getField();
`

func passiveRouteList() []crRoute {
	return []crRoute{
		{"a/cr.setCustomField(relationshipName)", passivePre + `cr.setCustomField('relationshipName', 'zzz');`},
		{"b/cr.setCustomField(zzz,1)", passivePre + `cr.setCustomField('zzz', 1);`},
		{"c/fld.setCustomField(field)", passivePre + `fld.setCustomField('field', 'zzz');`},
		{"d/cr.setError", passivePre + `cr.setError('a', 'b');`},
		{"d/fld.setError", passivePre + `fld.setError('a', 'b');`},
		{"f/withZzz", passivePre + `cr.withZzz(1);`},
		{"f/build", passivePre + `cr.build();`},
		{"f/add", passivePre + `cr.add(1);`},
		{"f/clear", passivePre + `cr.clear();`},
	}
}

func passiveFindJunction(t *testing.T, m *VM, api string) (string, int, []string) {
	m.currentMethod.APIVersion = api
	var names []string
	for k := range m.Org.Objects {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, o := range names {
		for i, r := range m.describeChildRelationships(o) {
			if l := r.Fields["junctionIdListNames"]; len(l.List) > 0 {
				var ns []string
				for _, v := range l.List {
					ns = append(ns, v.Text)
				}
				return o, i, ns
			}
		}
	}
	return "", -1, nil
}

func TestChildRelationshipDescribePassiveRoutes(t *testing.T) {
	routes := passiveRouteList()
	base0, tmpl0 := describeSharePrimedBaseFor(t, false)
	obj, idx, jn := passiveFindJunction(t, describeShareClone(base0, tmpl0), "67.0")
	digestOf := func(m *VM, api, o string) []string {
		m.currentMethod.APIVersion = api
		return crDigest(m.describeChildRelationships(o))
	}
	type rt struct {
		crRoute
		obj string
	}
	var all []rt
	for _, r := range routes {
		all = append(all, rt{r, "Account"})
	}
	if obj != "" {
		pre := fmt.Sprintf(`List<Schema.ChildRelationship> rels = Schema.getGlobalDescribe().get('%s').getDescribe().getChildRelationships(); Schema.ChildRelationship cr = rels[%d];
`, obj, idx)
		all = append(all,
			rt{crRoute{"e/remove existing", pre + fmt.Sprintf(`cr.removeJunctionIdListName('%s');`, jn[0])}, obj},
			rt{crRoute{"e/add zzz", pre + `cr.addJunctionIdListName('zzz');`}, obj},
			rt{crRoute{"e/remove then add", pre + fmt.Sprintf(`cr.removeJunctionIdListName('%s'); cr.addJunctionIdListName('zzz');`, jn[0])}, obj})
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range all {
			t.Run(api+"/"+route.name, func(t *testing.T) {
				base, template := describeSharePrimedBaseFor(t, false)
				early := describeShareClone(base, template)
				pristine := digestOf(early, api, route.obj)
				writer := describeShareClone(base, template)
				crRegister(t, writer, api)
				crRunWriter(t, writer, api, route.src)
				late := describeShareClone(base, template)
				leak := false
				for _, m := range []*VM{early, late} {
					if !reflect.DeepEqual(digestOf(m, api, route.obj), pristine) {
						leak = true
					}
				}
				if leak {
					t.Errorf("LEAK")
				}
			})
		}
	}
	for _, route := range all {
		t.Run("race/"+route.name, func(t *testing.T) {
			base, template := describeSharePrimedBaseFor(t, false)
			var group sync.WaitGroup
			start := make(chan struct{})
			for i := 0; i < 4; i++ {
				machine := describeShareClone(base, template)
				crRegister(t, machine, "67.0")
				reader := describeShareClone(base, template)
				group.Add(2)
				go func() { defer group.Done(); <-start; crRunWriter(t, machine, "67.0", route.src) }()
				go func() {
					defer group.Done()
					<-start
					for range 3 {
						digestOf(reader, "67.0", route.obj)
					}
				}()
			}
			close(start)
			group.Wait()
		})
	}
}

// A relationship with junction names keeps them in the shared values: removing
// and adding names through the passive accessors in one VM rewrites nothing the
// other VMs read, including the list backing array.
func TestChildRelationshipDescribeJunctionListsStayShared(t *testing.T) {
	// The shared fixture org carries no junction names, so add one explicitly.
	org := describeShareOrg()
	org.Objects["LinkParent__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{
		APIName: "LinkParent__c", Fields: map[string]storage.Field{"Id": {APIName: "Id", Label: "Record ID", Type: storage.FieldID}},
	}}
	org.Objects["LinkChild__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{
		APIName: "LinkChild__c",
		Fields: map[string]storage.Field{
			"Id":        {APIName: "Id", Label: "Record ID", Type: storage.FieldID},
			"Parent__c": {APIName: "Parent__c", Label: "Parent", Type: storage.FieldReference, ReferenceTo: []string{"LinkParent__c"}, RelationshipName: "Parent__r"},
		},
		Relations: []storage.Relationship{{
			Field:               "Parent__c",
			ParentObjects:       []string{"LinkParent__c"},
			ParentRelationship:  "Parent__r",
			ChildRelationship:   "Children__r",
			JunctionIDListNames: []string{"LinkIds"},
			JunctionReferenceTo: []string{"LinkParent__c"},
		}},
	}}
	base, template := describeSharePrimedBaseWithOrg(t, org)
	object, index, names := passiveFindJunction(t, describeShareClone(base, template), "67.0")
	if object == "" || len(names) == 0 {
		t.Fatal("fixture has no relationship with junction names")
	}
	read := func(machine *VM) []string {
		machine.currentMethod.APIVersion = "67.0"
		var out []string
		for _, item := range machine.describeChildRelationships(object)[index].Fields["junctionIdListNames"].List {
			out = append(out, item.Text)
		}
		return out
	}
	early := describeShareClone(base, template)
	if got := read(early); !reflect.DeepEqual(got, names) {
		t.Fatalf("junction names = %v, want %v", got, names)
	}
	writer := describeShareClone(base, template)
	describeShareRunApex(t, writer, fmt.Sprintf(`Schema.ChildRelationship cr = Schema.getGlobalDescribe().get('%s').getDescribe().getChildRelationships()[%d];
cr.removeJunctionIdListName('%s'); cr.addJunctionIdListName('zzz');`, object, index, names[0]))
	for name, machine := range map[string]*VM{"early": early, "late": describeShareClone(base, template)} {
		if got := read(machine); !reflect.DeepEqual(got, names) {
			t.Fatalf("%s VM junction names = %v, want %v", name, got, names)
		}
	}
}
