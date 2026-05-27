package vm

import (
	"fmt"
	"testing"
)

func BenchmarkExecTriggerDML(b *testing.B) {
	triggerProgram, err := CompileAnonymous(`
for (Account a : Trigger.new) {
	a.Name = a.Name + '!';
}
`)
	if err != nil {
		b.Fatal(err)
	}
	program, err := CompileAnonymous(`
for (Integer i = 0; i < 25; i++) {
	insert new Account(Name = 'Acme');
}
`)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		machine := New(nil)
		org := testDataOrg()
		machine.SetOrg(&org)
		if err := machine.RegisterTrigger(Trigger{
			Name:      "AccountBeforeInsert",
			Object:    "Account",
			Timing:    triggerTimingBefore,
			Operation: "insert",
			Program:   triggerProgram,
		}); err != nil {
			b.Fatal(err)
		}
		if _, err := machine.Execute(program); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCloneValuePreserveRefsLargeOrderGraph(b *testing.B) {
	root := Object("OrderGraph")
	lines := List()
	for i := 0; i < 200; i++ {
		line := Object("OrderLine")
		line.Fields["Name"] = String(fmt.Sprintf("line-%d", i))
		line.Fields["Price"] = Decimal(float64(i))
		line.Fields["Children"] = List(Object("Adjustment"), Object("Agreement"))
		lines.List = append(lines.List, line)
	}
	root.Fields["Lines"] = lines

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		cloned := cloneValuePreserveRefs(root)
		if cloned.Ref != root.Ref {
			b.Fatalf("clone lost root ref")
		}
	}
}

func BenchmarkReplaceValueAliasLargeOrderGraph(b *testing.B) {
	root := Object("OrderGraph")
	groups := List()
	for i := 0; i < 50; i++ {
		group := Object("OrderGroup")
		group.Fields["Name"] = String(fmt.Sprintf("group-%d", i))
		lines := List()
		for j := 0; j < 4; j++ {
			line := Object("OrderLine")
			line.Fields["Name"] = String(fmt.Sprintf("group-%d-line-%d", i, j))
			line.Fields["Children"] = List(Object("Adjustment"), Object("Agreement"))
			metadata := Map()
			priceKey := mapKey(String("price"))
			metadata.Map[priceKey] = Decimal(float64(i*10 + j))
			metadata.MapKeys[priceKey] = String("price")
			metadata.MapOrder = append(metadata.MapOrder, priceKey)
			line.Fields["Metadata"] = metadata
			lines.List = append(lines.List, line)
		}
		group.Fields["Lines"] = lines
		groups.List = append(groups.List, group)
	}

	previous := List()
	for i := 0; i < 200; i++ {
		previous.List = append(previous.List, Object("OrderLine"))
	}
	updated := previous
	updated.List = append(updated.List, Object("OrderLine"))
	targetGroup := Object("OrderGroup")
	targetGroup.Fields["Lines"] = previous
	groups.List = append(groups.List, targetGroup)
	root.Fields["Groups"] = groups

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		seen := make(map[uint64]bool)
		replaced, changed := replaceValueAlias(root, previous, updated, seen)
		replacedGroups := replaced.Fields["Groups"].List
		replacedLines := replacedGroups[len(replacedGroups)-1].Fields["Lines"].List
		if !changed || len(replacedLines) != 201 {
			b.Fatalf("alias was not replaced")
		}
	}
}

func BenchmarkReplaceAliasSnapshotLargeOrderGraph(b *testing.B) {
	root := Object("OrderGraph")
	groups := List()
	for i := 0; i < 50; i++ {
		group := Object("OrderGroup")
		group.Fields["Name"] = String(fmt.Sprintf("group-%d", i))
		lines := List()
		for j := 0; j < 4; j++ {
			line := Object("OrderLine")
			line.Fields["Name"] = String(fmt.Sprintf("group-%d-line-%d", i, j))
			line.Fields["Children"] = List(Object("Adjustment"), Object("Agreement"))
			metadata := Map()
			priceKey := mapKey(String("price"))
			metadata.Map[priceKey] = Decimal(float64(i*10 + j))
			metadata.MapKeys[priceKey] = String("price")
			metadata.MapOrder = append(metadata.MapOrder, priceKey)
			line.Fields["Metadata"] = metadata
			lines.List = append(lines.List, line)
		}
		group.Fields["Lines"] = lines
		groups.List = append(groups.List, group)
	}

	previous := List()
	for i := 0; i < 200; i++ {
		previous.List = append(previous.List, Object("OrderLine"))
	}
	updated := previous
	updated.List = append(updated.List, Object("OrderLine"))
	targetGroup := Object("OrderGroup")
	targetGroup.Fields["Lines"] = previous
	groups.List = append(groups.List, targetGroup)
	root.Fields["Groups"] = groups
	snapshot := snapshotAlias(previous)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		seen := make(map[uint64]bool)
		replaced, changed := replaceAliasSnapshot(root, snapshot, updated, seen)
		replacedGroups := replaced.Fields["Groups"].List
		replacedLines := replacedGroups[len(replacedGroups)-1].Fields["Lines"].List
		if !changed || len(replacedLines) != 201 {
			b.Fatalf("alias was not replaced")
		}
	}
}

func BenchmarkSameAliasRuntimeContentLargeOrderGraph(b *testing.B) {
	left := Object("OrderGraph")
	right := left
	for i := 0; i < 100; i++ {
		line := Object("OrderLine")
		line.Fields["Name"] = String(fmt.Sprintf("line-%d", i))
		line.Fields["Children"] = List(Object("Adjustment"), Object("Agreement"))
		left.Fields[fmt.Sprintf("Line%d", i)] = line
		right.Fields[fmt.Sprintf("Line%d", i)] = line
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if !sameAliasRuntimeData(left, right) {
			b.Fatal("runtime content mismatch")
		}
	}
}

func BenchmarkCollectStaticFieldValueRefsLargeOrderGraph(b *testing.B) {
	root := Object("OrderGraph")
	for i := 0; i < 100; i++ {
		line := Object("OrderLine")
		line.Fields["Name"] = String(fmt.Sprintf("line-%d", i))
		line.Fields["Children"] = List(Object("Adjustment"), Object("Agreement"))
		root.Fields[fmt.Sprintf("Line%d", i)] = line
	}
	location := staticFieldRef{ClassName: "OrderService", FieldName: "Graph"}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		refs := make(map[uint64]bool)
		fields := make(map[uint64][]staticFieldRef)
		collectStaticFieldValueRefs(root, refs, fields, location, make(map[uint64]bool))
		if !refs[root.Ref] {
			b.Fatal("root ref missing")
		}
	}
}

func BenchmarkTriggerNamespaceByName(b *testing.B) {
	machine := New(nil)
	machine.currentNamespace = "pkg"
	for i := 0; i < 500; i++ {
		if err := machine.RegisterTrigger(Trigger{
			Name:      fmt.Sprintf("Trigger%d", i),
			Object:    fmt.Sprintf("Object%d__c", i),
			Timing:    triggerTimingBefore,
			Operation: "insert",
		}); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if got := machine.triggerNamespaceByName("Trigger499"); got != "pkg" {
			b.Fatalf("trigger namespace = %q, want pkg", got)
		}
	}
}
