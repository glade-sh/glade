package apextest

import (
	"reflect"
	"testing"
)

func TestMethodTableSnapshotDetectsMutations(t *testing.T) {
	for _, tableName := range []string{"Methods", "MethodOverloads", "MethodFolded"} {
		for _, mutation := range []string{"entry", "parameter", "modifier", "program"} {
			t.Run(tableName+"/"+mutation, func(t *testing.T) {
				base := methodOverlayBaseMachine(t, 4)
				method := base.Methods["Lib0.call"]
				method.Modifiers = []string{"public"}
				base.Methods[method.Name] = method
				base.MethodOverloads[method.Name][3] = method
				base.MethodFolded["lib0.call"][3] = method
				table := reflect.ValueOf(base).Elem().FieldByName(tableName)
				before := snapshotMethodTable(table)
				unchanged := snapshotMethodTable(table)
				if !reflect.DeepEqual(before, unchanged) {
					t.Fatal("unchanged method table differs from its snapshot")
				}
				switch mutation {
				case "entry":
					table.SetMapIndex(table.MapKeys()[0], reflect.Value{})
				case "parameter":
					method.Params[0].Name = "changed"
				case "modifier":
					method.Modifiers[0] = "private"
				case "program":
					method.Program.Instructions[0].Expr.Value = "2"
				}
				if reflect.DeepEqual(before, snapshotMethodTable(table)) {
					t.Fatal("method table mutation was hidden by the snapshot")
				}
			})
		}
	}
}

func TestMethodTableSnapshotIncludesUnexportedState(t *testing.T) {
	type hiddenBody struct{ value string }
	type hiddenState struct {
		name    string
		names   []string
		params  map[string][]string
		body    *hiddenBody
		payload any
		fixed   [1]string
	}
	newTable := func() map[string]*hiddenState {
		return map[string]*hiddenState{"call": {
			name: "original", names: []string{"original"},
			params: map[string][]string{"p": {"String"}},
			body:   &hiddenBody{value: "original"}, payload: []string{"original"},
			fixed: [1]string{"original"},
		}}
	}
	mutations := map[string]func(*hiddenState){
		"scalar":        func(s *hiddenState) { s.name = "changed" },
		"slice":         func(s *hiddenState) { s.names[0] = "changed" },
		"map":           func(s *hiddenState) { s.params["new"] = []string{"Integer"} },
		"map slice":     func(s *hiddenState) { s.params["p"][0] = "Integer" },
		"pointer":       func(s *hiddenState) { s.body.value = "changed" },
		"interface":     func(s *hiddenState) { s.payload.([]string)[0] = "changed" },
		"dynamic type":  func(s *hiddenState) { s.payload = [1]string{"original"} },
		"array":         func(s *hiddenState) { s.fixed[0] = "changed" },
		"nil slice":     func(s *hiddenState) { s.names = nil },
		"empty slice":   func(s *hiddenState) { s.names = []string{} },
		"nil map":       func(s *hiddenState) { s.params = nil },
		"nil pointer":   func(s *hiddenState) { s.body = nil },
		"nil interface": func(s *hiddenState) { s.payload = nil },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			table := newTable()
			before := snapshotMethodTable(reflect.ValueOf(table))
			want := snapshotMethodTable(reflect.ValueOf(newTable()))
			if !reflect.DeepEqual(before, want) {
				t.Fatal("equal hidden state produced different snapshots")
			}
			mutate(table["call"])
			if !reflect.DeepEqual(before, want) {
				t.Fatal("snapshot retained a mutable alias to hidden state")
			}
			if reflect.DeepEqual(before, snapshotMethodTable(reflect.ValueOf(table))) {
				t.Fatal("hidden state mutation was omitted from the snapshot")
			}
		})
	}
}
