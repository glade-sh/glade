package vm

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
)

// every route below runs Apex in a writer VM, then checks an
// early VM, a late VM and a private build keep the pre-write digest.

func crDump(v Value) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("<%v|%s|%s|%v|%v", v.Kind, v.Type, v.Text, v.Bool, v.Int))
	keys := make([]string, 0, len(v.Fields))
	for k := range v.Fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		b.WriteString(";" + k + "=" + crDump(v.Fields[k]))
	}
	for _, item := range v.List {
		b.WriteString("," + crDump(item))
	}
	b.WriteString(">")
	return b.String()
}

func crDigest(values []Value) []string {
	out := make([]string, len(values))
	for i, v := range values {
		out[i] = crDump(v)
	}
	return out
}

// crRels lists Account child relationships as the given API version sees them
// (the cache key includes the API version, so the digest must use the writer's).
func crRels(t *testing.T, machine *VM, api string) []Value {
	t.Helper()
	machine.currentMethod.APIVersion = api
	return accountChildRelationships(t, machine)
}

type crRoute struct{ name, src string }

const crGet = `List<Schema.ChildRelationship> rels = Schema.SObjectType.Account.getDescribe().getChildRelationships();
Schema.ChildRelationship cr = rels[1];
Schema.SObjectType tok = cr.getChildSObject();
Schema.SObjectField fld = cr.getField();
`

func crRoutes() []crRoute {
	r := []crRoute{}
	add := func(n, s string) { r = append(r, crRoute{n, crGet + s}) }
	// 1 passive accessors
	for _, p := range []struct{ recv, label string }{{"cr", "cr"}, {"tok", "tok"}, {"fld", "fld"}, {"cr.getChildSObject()", "crTokCall"}, {"cr.getField()", "crFldCall"}} {
		add("1/"+p.label+"/setZzz", p.recv+".setZzz(1);")
		add("1/"+p.label+"/addZzz1", p.recv+".addZzz(1);")
		add("1/"+p.label+"/addZzz2", p.recv+".addZzz('k',1);")
		add("1/"+p.label+"/removeZzz", p.recv+".removeZzz(1);")
	}
	add("1/cr/setRelationshipName", `cr.setRelationshipName('zzz');`)
	add("1/tok/setObject", `tok.setObject('Contact');`)
	add("1/fld/setField", `fld.setField('zzz');`)
	add("1/crTokCall/setObject", `cr.getChildSObject().setObject('Contact');`)
	// 2 direct field writes
	add("2/cr.relationshipName", `cr.relationshipName = 'zzz';`)
	add("2/cr.zzz", `cr.zzz = 1;`)
	add("2/cr.childSObject.object", `cr.childSObject.object = 'Contact';`)
	add("2/getChildSObject().object local", `Schema.SObjectType t2 = cr.getChildSObject(); t2.object = 'Contact';`)
	add("2/getChildSObject().object direct", `cr.getChildSObject().object = 'Contact';`)
	add("2/rels[0].field.name", `rels[0].field.name = 'zzz';`)
	add("2/rels[0].field.field", `rels[0].field.field = 'zzz';`)
	add("2/rels[0].field.object", `rels[0].field.object = 'zzz';`)
	// 3 holders
	const holderInit = `Holder h = new Holder(); h.rels = rels; h.objs = (List<Object>) rels; h.m = new Map<String,Object>{'k' => rels}; Holder.srels = rels;
`
	holderWrites := map[string]string{
		"inst.relName":           `h.rels[1].relationshipName = 'zzz';`,
		"inst.childObj":          `h.rels[1].childSObject.object = 'Contact';`,
		"inst.fieldName":         `h.rels[1].field.name = 'zzz';`,
		"static.relName":         `Holder.srels[1].relationshipName = 'zzz';`,
		"static.childObj":        `Holder.srels[1].childSObject.object = 'Contact';`,
		"mapval.relName":         `((List<Schema.ChildRelationship>) h.m.get('k'))[1].relationshipName = 'zzz';`,
		"mapval.childObj":        `((List<Schema.ChildRelationship>) h.m.get('k'))[1].childSObject.object = 'Contact';`,
		"listobj.relName":        `((Schema.ChildRelationship) h.objs[1]).relationshipName = 'zzz';`,
		"listobj.childObj":       `((Schema.ChildRelationship) h.objs[1]).childSObject.object = 'Contact';`,
		"tokenLocal.held":        `Schema.SObjectType t3 = rels[1].getChildSObject(); t3.object='Contact';`,
		"tokenLocal.heldInst":    `Schema.SObjectType t3 = h.rels[1].getChildSObject(); t3.object='Contact';`,
		"tokenLocal.heldStatic":  `Schema.SObjectType t3 = Holder.srels[1].getChildSObject(); t3.object='Contact'; Schema.SObjectField f3 = Holder.srels[1].getField(); f3.field='zzz';`,
		"tokenLocal.heldMap":     `Schema.SObjectType t3 = ((List<Schema.ChildRelationship>) h.m.get('k'))[1].getChildSObject(); t3.object='Contact';`,
		"tokenLocal.heldObjs":    `Schema.SObjectType t3 = ((Schema.ChildRelationship) h.objs[1]).getChildSObject(); t3.object='Contact';`,
		"crLocal.held":           `Schema.ChildRelationship c3 = h.rels[1]; c3.relationshipName='zzz'; Schema.ChildRelationship c4 = Holder.srels[2]; c4.relationshipName='zzz2';`,
		"tokenLocal.heldPassive": `Schema.SObjectType t3 = h.rels[1].getChildSObject(); t3.setObject('Contact'); t3.setZzz(1);`,
		"crLocal.heldPassive":    `Schema.ChildRelationship c3 = h.rels[1]; c3.setZzz(1); c3.setRelationshipName('zzz');`,
	}
	names := make([]string, 0)
	for k := range holderWrites {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, k := range names {
		add("3/"+k, holderInit+holderWrites[k])
	}
	// 4 other
	add("4/JSON.serialize", `String s = JSON.serialize(rels); System.assert(s.length() > 10);`)
	add("4/JSON.serialize one", `String s = JSON.serialize(cr);`)
	add("4/sort", `rels.sort();`)
	add("4/clone then write", `Schema.ChildRelationship c = (Schema.ChildRelationship) cr.clone(); c.relationshipName = 'zzz';`)
	add("4/clone then passive write", `Schema.ChildRelationship c = (Schema.ChildRelationship) cr.clone(); c.setZzz(1); c.setRelationshipName('zzz');`)
	add("4/clone tok then write", `Schema.SObjectType c = (Schema.SObjectType) tok.clone(); c.object = 'Contact';`)
	add("4/list clone write", `List<Schema.ChildRelationship> c = rels.clone(); c[1].relationshipName = 'zzz'; c[1].childSObject.object='Contact';`)
	add("4/equals", `Boolean b = rels[0].equals(rels[1]);`)
	add("4/hashCode", `Integer i = rels[0].hashCode();`)
	add("4/map key", `Map<Schema.ChildRelationship,Integer> m = new Map<Schema.ChildRelationship,Integer>{rels[0] => 1, rels[1] => 2}; m.put(rels[0], 3);`)
	add("4/set element", `Set<Schema.ChildRelationship> s = new Set<Schema.ChildRelationship>(rels); s.add(rels[0]); s.remove(rels[1]);`)
	add("4/String.valueOf", `String s = String.valueOf(rels[0]) + String.valueOf(rels[0].getChildSObject());`)
	add("4/System.debug", `System.debug(rels); System.debug(rels[0]);`)
	add("4/junctionIdListNames add", `cr.getJunctionIdListNames().add('x'); rels[0].getJunctionIdListNames().add('x');`)
	add("4/junctionReferenceTo clear", `cr.getJunctionReferenceTo().clear(); rels[0].getJunctionReferenceTo().clear();`)
	add("4/junctionIdListNames field add", `cr.junctionIdListNames.add('x');`)
	add("4/junctionReferenceTo field clear", `cr.junctionReferenceTo.clear();`)
	add("4/for-each local write", `for (Schema.ChildRelationship each : rels) { Schema.SObjectType t = each.getChildSObject(); t.object = 'Contact'; }`)
	add("4/queueable", `Test.startTest(); QWorker q = new QWorker(); q.rels = rels; q.tok = tok; System.enqueueJob(q); Test.stopTest();`)
	add("4/queueable no test ctx", `QWorker q = new QWorker(); q.rels = rels; System.enqueueJob(q);`)
	return r
}

func crRegister(t *testing.T, machine *VM, api string) {
	t.Helper()
	if err := machine.RegisterClass(Class{Name: "Holder", APIVersion: api,
		Fields: map[string]Field{
			"rels": {Name: "rels", Type: "List<Schema.ChildRelationship>"},
			"objs": {Name: "objs", Type: "List<Object>"},
			"m":    {Name: "m", Type: "Map<String,Object>"},
		},
		FieldOrder:   []string{"rels", "objs", "m"},
		StaticFields: map[string]Field{"srels": {Name: "srels", Type: "List<Schema.ChildRelationship>", Static: true}}}); err != nil {
		t.Fatal(err)
	}
	prog, err := CompileAnonymousWithOptions(`System.debug(rels.size()); rels[1].relationshipName = 'qw'; rels[1].childSObject.object = 'Contact'; rels[1].setZzz(1); tok.object = 'ContactQ'; rels.clear();`, CompileOptions{APIVersion: api})
	if err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "QWorker", APIVersion: api, Interfaces: []string{"Queueable"},
		Fields: map[string]Field{
			"rels": {Name: "rels", Type: "List<Schema.ChildRelationship>"},
			"tok":  {Name: "tok", Type: "Schema.SObjectType"},
		}, FieldOrder: []string{"rels", "tok"},
		Methods: map[string]Method{"execute": {Name: "QWorker.execute", ClassName: "QWorker", APIVersion: api, Params: []Param{{Name: "context", Type: "QueueableContext"}}, Program: prog}}}); err != nil {
		t.Fatal(err)
	}
}

func crRunWriter(t *testing.T, machine *VM, api, src string) (status string) {
	program, err := CompileAnonymousWithOptions(src, CompileOptions{APIVersion: api})
	if err != nil {
		return "COMPILE-ERR: " + firstLine(err.Error())
	}
	result, err := machine.Execute(program)
	if err != nil {
		return "EXEC-ERR: " + firstLine(err.Error())
	}
	if derr := machine.DrainAsync(&result); derr != nil {
		return "DRAIN-ERR: " + firstLine(derr.Error())
	}
	return "EXECUTED"
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if len(s) > 160 {
		s = s[:160]
	}
	return s
}

func TestChildRelationshipDescribeRoutesKeepSharedValues(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range crRoutes() {
			t.Run(api+"/"+route.name, func(t *testing.T) {
				base, template := describeSharePrimedBaseFor(t, false)
				early := describeShareClone(base, template)
				pristine := crDigest(crRels(t, early, api))
				writer := describeShareClone(base, template)
				crRegister(t, writer, api)
				status := crRunWriter(t, writer, api, route.src)
				late := describeShareClone(base, template)
				for name, machine := range map[string]*VM{"early": early, "late": late} {
					if got := crDigest(crRels(t, machine, api)); !reflect.DeepEqual(got, pristine) {
						t.Errorf("LEAK: %s VM sees changed child relationships (status %s)", name, status)
					}
				}
			})
		}
	}
}

// Race check: every route concurrently on clones of one base, with readers.
func TestChildRelationshipDescribeRoutesConcurrent(t *testing.T) {
	base, template := describeSharePrimedBaseFor(t, false)
	pristine := crDigest(crRels(t, describeShareClone(base, template), "67.0"))
	var group sync.WaitGroup
	start := make(chan struct{})
	routes := crRoutes()
	for i, route := range routes {
		machine := describeShareClone(base, template)
		crRegister(t, machine, "67.0")
		group.Add(1)
		go func(route crRoute, machine *VM) {
			defer group.Done()
			<-start
			crRunWriter(t, machine, "67.0", route.src)
		}(route, machine)
		if i%3 == 0 {
			reader := describeShareClone(base, template)
			group.Add(1)
			go func() {
				defer group.Done()
				<-start
				for range 3 {
					crDigest(crRels(t, reader, "67.0"))
				}
			}()
		}
	}
	close(start)
	group.Wait()
	if got := crDigest(crRels(t, describeShareClone(base, template), "67.0")); !reflect.DeepEqual(got, pristine) {
		t.Fatal("LEAK: shared child relationships changed after concurrent routes")
	}
}

// Race check per route: three writers of one route plus a reader, one base.
func TestChildRelationshipDescribeRoutesConcurrentEach(t *testing.T) {
	for _, route := range crRoutes() {
		t.Run(route.name, func(t *testing.T) {
			base, template := describeSharePrimedBaseFor(t, false)
			pristine := crDigest(crRels(t, describeShareClone(base, template), "67.0"))
			var group sync.WaitGroup
			start := make(chan struct{})
			for i := 0; i < 4; i++ {
				machine := describeShareClone(base, template)
				crRegister(t, machine, "67.0")
				group.Add(2)
				go func() {
					defer group.Done()
					<-start
					crRunWriter(t, machine, "67.0", route.src)
				}()
				reader := describeShareClone(base, template)
				go func() {
					defer group.Done()
					<-start
					for range 3 {
						crDigest(crRels(t, reader, "67.0"))
					}
				}()
			}
			close(start)
			group.Wait()
			if got := crDigest(crRels(t, describeShareClone(base, template), "67.0")); !reflect.DeepEqual(got, pristine) {
				t.Fatal("LEAK after concurrent route")
			}
		})
	}
}
