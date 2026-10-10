package vm

import (
	"fmt"
	"testing"
)

func TestMapRemoveAliasDetachedHolders(t *testing.T) {
	for _, receiver := range []struct {
		name string
		expr string
	}{
		{"local", "local"},
		{"static", "MapRemoveAliasState.Shared"},
		{"instance", "holder.Items"},
		{"list", "((Map<String,Integer>)objects[0])"},
	} {
		t.Run(receiver.name, func(t *testing.T) {
			machine, _ := runDynamicObjectAliasProgram(t, `
Map<String,Integer> target = new Map<String,Integer>{'removed' => 1, 'kept' => 2};
Map<String,Integer> local = target;
MapRemoveAliasState.Shared = target;
MapRemoveAliasState holder = new MapRemoveAliasState();
holder.Items = target;
List<Object> objects = new List<Object>{target};
`, Class{
				Name: "MapRemoveAliasState",
				Fields: map[string]Field{
					"Items": {Name: "Items", Type: "Map<String,Integer>"},
				},
				StaticFields: map[string]Field{
					"Shared": {Name: "Shared", Type: "Map<String,Integer>", Static: true},
				},
			})
			ref := machine.Globals["target"].Ref
			for _, mutation := range []struct {
				source string
				size   int
			}{
				{fmt.Sprintf("Integer removed = %s.remove('removed'); System.assertEquals(1, removed);", receiver.expr), 1},
				{receiver.expr + ".clear();", 0},
			} {
				// Separate every holder's Go storage while retaining the Apex map
				// identity. Shared backing maps must not hide missed propagation.
				for name, value := range machine.Globals {
					machine.Globals[name] = cloneValuePreserveRefs(value)
				}
				class := machine.Classes["MapRemoveAliasState"]
				field := class.StaticFields["Shared"]
				field.Value = cloneValuePreserveRefs(field.Value)
				class.StaticFields["Shared"] = field
				mapRemoveAliasExecute(t, machine, mutation.source)

				// Check raw storage before an Apex read can refresh a stale alias.
				for name, alias := range map[string]Value{
					"original": machine.Globals["target"],
					"local":    machine.Globals["local"],
					"static":   class.StaticFields["Shared"].Value,
					"field":    machine.Globals["holder"].Fields["Items"],
					"list":     machine.Globals["objects"].List[0],
				} {
					if alias.Kind != ValueMap || alias.Ref != ref {
						t.Errorf("%s: %s has kind=%s Ref=%d, want map Ref=%d", mutation.source, name, alias.Kind, alias.Ref, ref)
					}
					if len(alias.Map) != mutation.size {
						t.Errorf("%s: %s size=%d, want %d", mutation.source, name, len(alias.Map), mutation.size)
					}
					if _, present := alias.Map[mapKey(String("removed"))]; present {
						t.Errorf("%s: %s retained removed key", mutation.source, name)
					}
					if mutation.size == 1 && !alias.Map[mapKey(String("kept"))].Equal(Int(2)) {
						t.Errorf("%s: %s lost kept value", mutation.source, name)
					}
				}
				mapRemoveAliasExecute(t, machine, fmt.Sprintf(`
System.assertEquals(%[1]d, target.size(), 'original');
System.assertEquals(%[1]d, local.size(), 'local');
System.assertEquals(%[1]d, MapRemoveAliasState.Shared.size(), 'static');
System.assertEquals(%[1]d, holder.Items.size(), 'field');
System.assertEquals(%[1]d, ((Map<String,Integer>)objects[0]).size(), 'List<Object>');
`, mutation.size))
			}
		})
	}
}

func mapRemoveAliasExecute(t *testing.T, machine *VM, source string) {
	t.Helper()
	program, err := CompileAnonymous(source)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatalf("%s: %v", source, err)
	}
}
