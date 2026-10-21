package vm

import "testing"

// CompileAnonymous lowers these method-body fragments for focused compiler
// checks. Native route coverage separately invokes the captured helper method;
// it does not establish direct anonymous declaration behavior.
func TestTransientLocalDeclarationValues(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, tc := range []struct {
			name, body string
		}{
			{
				name: "map assigned after declaration",
				body: `transient Map<String,Object> values; values=new Map<String,Object>{'slot'=>1}; System.assertEquals(1,values.get('slot'));`,
			},
			{
				name: "initialized integer",
				body: `transient Integer quantity=3; System.assertEquals(3,quantity);`,
			},
			{
				name: "ordinary map control",
				body: `Map<String,Object> values=new Map<String,Object>{'slot'=>4}; System.assertEquals(4,values.get('slot'));`,
			},
			{
				name: "final integer control",
				body: `final Integer quantity=4; System.assertEquals(4,quantity);`,
			},
		} {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				program, err := CompileAnonymousWithOptions(tc.body, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := Execute(program, nil); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
