package vm

import (
	"reflect"
	"testing"
)

// This test checks the candidate's local order bookkeeping, rather than
// claiming an uncaptured Salesforce collection-order guarantee.
func TestRestParameterOrderBookkeeping(t *testing.T) {
	for _, names := range [][]string{{"zebra", "alpha"}, {"alpha", "zebra"}} {
		t.Run(names[0], func(t *testing.T) {
			request := Object("RestRequest")
			request.Fields["params"] = typedMap("Map<String,String>")
			want := []string{mapKey(String(names[0])), mapKey(String(names[1]))}
			for i, name := range names {
				_, updated, mutated, handled, err := callRestRequestMember(request, "addParameter", []Value{String(name), String(Int(int64(i)).String())})
				if err != nil || !handled || !mutated {
					t.Fatalf("addParameter: handled=%v mutated=%v error=%v", handled, mutated, err)
				}
				request = updated
			}
			if got := request.Fields["params"].MapOrder; !reflect.DeepEqual(got, want) {
				t.Fatalf("new keys order=%v want=%v", got, want)
			}
			_, request, _, _, err := callRestRequestMember(request, "addParameter", []Value{String(names[0]), String("updated")})
			if err != nil {
				t.Fatal(err)
			}
			params := request.Fields["params"]
			if !reflect.DeepEqual(params.MapOrder, want) || len(params.Map) != 2 || params.Map[want[0]].Text != "updated" {
				t.Fatalf("overwrite changed key position or failed to replace value: %#v", params)
			}
		})
	}
}

func TestRestParameterOrderThroughAlias(t *testing.T) {
	program, err := CompileAnonymous(`
RestRequest request = new RestRequest();
Map<String,String> alias = request.params;
request.addParameter('zebra', 'first');
request.addParameter('alpha', 'second');
request.addParameter('zebra', 'updated');
System.assertEquals('zebra,alpha', String.join(new List<String>(request.params.keySet()), ','));
System.assert(alias.keySet().contains('zebra'));
System.assert(alias.keySet().contains('alpha'));
System.assertEquals('updated', alias.get('zebra'));
System.assertEquals('second', alias.get('alpha'));
System.assertEquals(2, alias.size());
Map<String,String> ordinary = new Map<String,String>();
ordinary.put('zebra', 'first');
ordinary.put('alpha', 'second');
System.assertEquals('zebra,alpha', String.join(new List<String>(ordinary.keySet()), ','));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}
