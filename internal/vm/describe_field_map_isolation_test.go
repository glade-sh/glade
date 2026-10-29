package vm

import (
	"fmt"
	"reflect"
	"sync"
	"testing"
)

// Local-baseline controls for field-map holders that share frozen storage.
// Expected debug lines were produced by the eager-copy implementation; native
// read anchors remain testdata/describe_sharing/org.tsv:2,4,5,7. Container
// writes on these maps are read-only, so the observable writes are
// entry-changing coercions (an unqualified SObjectField view, the internal
// DescribeFieldResult conversion) and field-token writes, which stay shallowly
// shared within one VM.
var fieldMapHolderApexScenarios = map[string]struct {
	source string
	debug  []string
}{
	"unqualified": {
		source: `Map<String,Schema.SObjectField> a = Account.SObjectType.getDescribe().fields.getMap();
Map<String,Schema.SObjectField> b = Account.SObjectType.getDescribe().fields.getMap();
Map<String,Schema.SObjectField> alias = a;
Map<String,SObjectField> u = a;
System.debug(u === a); System.debug(alias === a); System.debug(a === b); System.debug(a == b);
System.debug(a.get('Name')); System.debug(b.get('Name')); System.debug(String.valueOf(alias.get('Name')));
System.debug(a.size()+' '+b.size()+' '+u.size());
System.debug(String.valueOf(u).length()); System.debug(String.valueOf(b).length());
Map<String,Schema.SObjectField> c = Account.SObjectType.getDescribe().fields.getMap();
System.debug(c === a); System.debug(c.get('Name') == a.get('Name'));`,
		debug: []string{"true", "true", "false", "true", "Name", "Name", "Name", "62 62 62", "7552", "7552", "false", "true"},
	},
	"describeCast": {
		source: `Map<String,Schema.SObjectField> a = Account.SObjectType.getDescribe().fields.getMap();
Map<String,Schema.SObjectField> b = Account.SObjectType.getDescribe().fields.getMap();
Map<String,Schema.SObjectField> alias = a;
Object o = a;
try { Map<String,Schema.DescribeFieldResult> d = (Map<String,Schema.DescribeFieldResult>) o; System.debug(d === a); } catch (Exception e) { System.debug(e.getTypeName()); }
Map<String,Object> om = a; System.debug(om === a);
Object x = alias.get('Name'); System.debug(x instanceof Schema.DescribeFieldResult); System.debug(x instanceof Schema.SObjectField);
Object y = b.get('Name'); System.debug(y instanceof Schema.SObjectField); System.debug(y instanceof Schema.DescribeFieldResult);
Map<String,Schema.SObjectField> c = Account.SObjectType.getDescribe().fields.getMap();
Object z = c.get('Name'); System.debug(z instanceof Schema.SObjectField);
System.debug(String.valueOf(alias).length()); System.debug(String.valueOf(b).length()); System.debug(JSON.serialize(alias).length());
System.debug(alias.keySet().size()+' '+alias.values().size()+' '+alias.clone().size());`,
		debug: []string{"System.TypeException", "true", "false", "true", "true", "false", "true", "7552", "7552", "32541", "62 62 62"},
	},
	"arguments": {
		source: `Map<String,Schema.SObjectField> a = Account.SObjectType.getDescribe().fields.getMap();
Map<String,Schema.SObjectField> b = Account.SObjectType.getDescribe().fields.getMap();
List<Object> l = new List<Object>{a, (Map<String,SObjectField>) a, b};
System.debug(l.size()); System.debug(l[0] === l[1]); System.debug(l[0] === l[2]);
Map<String,Object> m = new Map<String,Object>{'a' => a, 'u' => (Map<String,SObjectField>) a};
System.debug(m.get('a') === m.get('u'));
Map<String,Schema.SObjectField> merged = new Map<String,Schema.SObjectField>(a); merged.putAll(b); merged.put('extra', Account.Name);
System.debug(merged.size()+' '+a.size()+' '+b.size());
System.debug(String.valueOf(l).length());`,
		debug: []string{"3", "true", "false", "true", "249 62 62", "22662"},
	},
	"heap": {
		source: `Integer h0 = Limits.getHeapSize();
Map<String,Schema.SObjectField> a = Account.SObjectType.getDescribe().fields.getMap();
Integer h1 = Limits.getHeapSize();
Map<String,Schema.SObjectField> b = Account.SObjectType.getDescribe().fields.getMap();
Integer h2 = Limits.getHeapSize();
Map<String,SObjectField> u = a;
Integer h3 = Limits.getHeapSize();
System.debug((h1-h0)+' '+(h2-h1)+' '+(h3-h2));`,
		debug: []string{"13823 5671 11"},
	},
	"mutations": {
		source: `Map<String,Schema.SObjectField> a = Account.SObjectType.getDescribe().fields.getMap();
Map<String,Schema.SObjectField> b = Account.SObjectType.getDescribe().fields.getMap();
Set<String> keys = a.keySet(); keys.remove('name'); keys.clear();
System.debug(a.size()+' '+b.size()+' '+keys.size());
Map<String,Schema.SObjectField> cp = a.clone(); System.debug(cp === a); System.debug(cp.size());
System.debug(a.containsKey('Name')+' '+b.containsKey('name')+' '+a.containsKey('Account.Name'));
for (String k : new List<String>(b.keySet())) { if (k == 'name') System.debug('found'); }`,
		debug: []string{"62 62 0", "false", "62", "true true false", "found"},
	},
}

func TestFieldMapHolderApexRoutesMatchEagerBaseline(t *testing.T) {
	for name, scenario := range fieldMapHolderApexScenarios {
		for _, api := range []string{"62.0", "67.0"} {
			program, err := CompileAnonymousWithOptions(scenario.source, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			for _, namespaced := range []bool{false, true} {
				for _, primed := range []bool{false, true} {
					t.Run(fmt.Sprintf("%s/%s/namespace=%v/primed=%v", name, api, namespaced, primed), func(t *testing.T) {
						machine := describeShareUnprimedFor(namespaced)
						if primed {
							base, template := describeSharePrimedBaseFor(t, namespaced)
							machine = describeShareClone(base, template)
						}
						result, err := machine.Execute(program)
						if err != nil || !reflect.DeepEqual(result.Debug, scenario.debug) {
							t.Fatalf("err=%v\n debug=%q\n want %q", err, result.Debug, scenario.debug)
						}
					})
				}
			}
		}
	}
}

func fieldMapEntry(t *testing.T, machine *VM, holder Value) Value {
	t.Helper()
	value, _, err := machine.callValueMember("", holder, "get", []Value{String("Name")}, &Result{})
	if err != nil {
		t.Fatal(err)
	}
	return value
}

// Each holder owns its descriptor. A detach of one holder reaches its true
// aliases (scope, nested and Go-local copies) through the read boundary, and
// never a sibling, the wrapper's frozen map, or a later getMap result.
func TestFieldMapHolderDetachReachesOnlyTrueAliases(t *testing.T) {
	for _, target := range []string{"Map<String,SObjectField>", "Map<String,Schema.DescribeFieldResult>"} {
		for _, primed := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/primed=%v", target, primed), func(t *testing.T) {
				machine := describeShareUnprimed()
				if primed {
					base, template := describeSharePrimedBase(t)
					machine = describeShareClone(base, template)
				}
				wrapper := fieldMapWrapper(t, machine, "Account")
				first := fieldMapGet(t, machine, wrapper)
				second := fieldMapGet(t, machine, wrapper)
				owner := Object("FieldMapHolder")
				owner.Fields["fields"] = first
				machine.Globals["alias"] = first
				machine.Globals["nested"] = List(owner)
				pristine := fieldMapEntry(t, machine, second)
				converted, err := machine.coerceAssignable(target, first)
				if err != nil || converted.Ref != first.Ref {
					t.Fatalf("coercion: ref preserved=%v err=%v", converted.Ref == first.Ref, err)
				}
				want := converted.Map[mapKey(String("Name"))]
				if reflect.DeepEqual(want, pristine) {
					t.Fatal("target did not change the entry representation")
				}
				for label, alias := range map[string]Value{"local": first, "scope": machine.Globals["alias"], "nested": machine.Globals["nested"].List[0].Fields["fields"]} {
					if got := fieldMapEntry(t, machine, alias); !reflect.DeepEqual(got, want) {
						t.Fatalf("%s alias missed its holder's detach: %+v", label, got)
					}
				}
				source, _ := machine.privateDescribeFieldMap(wrapper)
				later := fieldMapGet(t, machine, wrapper)
				for label, independent := range map[string]Value{"sibling": second, "source": source, "later": later} {
					if got := fieldMapEntry(t, machine, independent); !reflect.DeepEqual(got, pristine) {
						t.Fatalf("detach reached %s storage: %+v", label, got)
					}
				}
				if descriptor := fieldMapHolderDescriptor(second); descriptor == nil || descriptor.value.Map != nil {
					t.Fatal("a sibling descriptor recorded another holder's detach")
				}
				if !sameMapBacking(later.Map, source.Map) || !later.mapBackingShared {
					t.Fatal("the frozen source stopped serving later holders")
				}
			})
		}
	}
}

// The wrapper's own map can escape through its proxy routes before any getMap.
// Freezing it at the privateDescribeFieldMap gate keeps an entry-changing
// write through that escape away from existing holders, while the wrapper and
// later holders observe it as the eager implementation's in-place rewrite did.
func TestFieldMapSourceDetachKeepsEarlierHolders(t *testing.T) {
	for _, target := range []string{"Map<String,SObjectField>", "Map<String,Schema.DescribeFieldResult>"} {
		t.Run(target, func(t *testing.T) {
			machine := describeShareUnprimed()
			wrapper := fieldMapWrapper(t, machine, "Account")
			earlier := fieldMapGet(t, machine, wrapper)
			pristine := fieldMapEntry(t, machine, earlier)
			escaped, _ := machine.privateDescribeFieldMap(wrapper)
			if !escaped.mapBackingShared || fieldMapHolderDescriptor(escaped) == nil {
				t.Fatal("the wrapper map escaped before it was frozen")
			}
			converted, err := machine.coerceAssignable(target, escaped)
			if err != nil || converted.Ref != escaped.Ref {
				t.Fatalf("coercion: %v", err)
			}
			want := converted.Map[mapKey(String("Name"))]
			if got := fieldMapEntry(t, machine, earlier); !reflect.DeepEqual(got, pristine) {
				t.Fatalf("source detach reached an earlier holder: %+v", got)
			}
			again, _ := machine.privateDescribeFieldMap(wrapper)
			later := fieldMapGet(t, machine, wrapper)
			for label, observer := range map[string]Value{"escaped": escaped, "wrapper": again, "later": later} {
				if got := fieldMapEntry(t, machine, observer); !reflect.DeepEqual(got, want) {
					t.Fatalf("%s missed the wrapper map's own rewrite: %+v", label, got)
				}
			}
			if again.Ref != escaped.Ref || later.Ref == escaped.Ref {
				t.Fatal("wrapper identity or later holder identity changed")
			}
		})
	}
}

// A failed coercion has no successful prefix and leaves every participant on
// the frozen storage.
func TestFieldMapFailedCoercionKeepsFrozenStorage(t *testing.T) {
	machine := describeShareUnprimed()
	wrapper := fieldMapWrapper(t, machine, "Account")
	first := fieldMapGet(t, machine, wrapper)
	second := fieldMapGet(t, machine, wrapper)
	before := privateDescribeCollection(first)
	if _, err := machine.coerceAssignable("Map<String,Integer>", first); err == nil {
		t.Fatal("wrong entry type converted")
	}
	for _, holder := range []Value{first, second} {
		current := holder.currentMapKeySet()
		if !reflect.DeepEqual(current.Map, before.Map) || !current.mapBackingShared || fieldMapHolderDescriptor(holder).value.Map != nil {
			t.Fatal("failed coercion detached or changed a holder")
		}
	}
}

// Field tokens keep the eager lifetime: a write is visible to every holder in
// the writer VM, later executions in it and, until SetOrg, the wrapper; a new
// org binding, another clone and a holder kept from before are independent.
func TestFieldMapHolderTokenLifetime(t *testing.T) {
	for _, primed := range []bool{false, true} {
		t.Run(fmt.Sprint(primed), func(t *testing.T) {
			base, template := describeSharePrimedBase(t)
			machine := describeShareUnprimed()
			if primed {
				machine = describeShareClone(base, template)
			}
			describeShareRunApex(t, machine, `Map<String,Schema.SObjectField> a = Account.SObjectType.getDescribe().fields.getMap();
Schema.SObjectField t = a.get('Name'); t.label = 'writer';
System.assertEquals('writer', Account.SObjectType.getDescribe().fields.getMap().get('Name').label);`)
			describeShareRunApex(t, machine, `System.assertEquals('writer', Account.SObjectType.getDescribe().fields.getMap().get('Name').label);`)
			describeShareRunApex(t, describeShareClone(base, template), `System.assertEquals('Account Name', Account.SObjectType.getDescribe().fields.getMap().get('Name').label);`)
			kept := fieldMapGet(t, machine, fieldMapWrapper(t, machine, "Account"))
			machine.SetOrg(machine.Org)
			describeShareRunApex(t, machine, `System.assertEquals('Account Name', Account.SObjectType.getDescribe().fields.getMap().get('Name').label);`)
			if label := fieldMapEntry(t, machine, kept).Fields["label"].Text; label != "writer" {
				t.Fatalf("a holder kept across SetOrg lost its own token: %q", label)
			}
		})
	}
}

// Concurrent clones of one primed base each freeze a VM-private copy; holder
// detaches and token writes stay inside their clone. Run with -race.
func TestFieldMapHoldersConcurrentClonesStayPrivate(t *testing.T) {
	base, template := describeSharePrimedBase(t)
	var wait sync.WaitGroup
	errors := make(chan error, 8)
	for worker := range 8 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			machine := describeShareClone(base, template)
			label := fmt.Sprintf("clone %d", worker)
			program, err := CompileAnonymous(fmt.Sprintf(`Map<String,Schema.SObjectField> a = Account.SObjectType.getDescribe().fields.getMap();
Map<String,Schema.SObjectField> b = Account.SObjectType.getDescribe().fields.getMap();
Map<String,SObjectField> u = a;
Schema.SObjectField t = u.get('Name'); t.label = '%s';
System.assertEquals('%s', b.get('Name').label);
System.assertEquals(a.size(), b.size());`, label, label))
			if err == nil {
				_, err = machine.Execute(program)
			}
			if err != nil {
				errors <- err
			}
		}()
	}
	wait.Wait()
	close(errors)
	for err := range errors {
		t.Fatal(err)
	}
	describeShareRunApex(t, describeShareClone(base, template), `System.assertEquals('Account Name', Account.SObjectType.getDescribe().fields.getMap().get('Name').label);`)
}
