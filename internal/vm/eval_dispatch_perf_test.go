package vm

import (
	"fmt"
	"testing"
)

// Keep recursive scalar evaluation allocation-free while call expressions use
// the same evaluator for their operands. These are owned arithmetic controls,
// not additional claims about Salesforce behavior.
func TestRecursiveScalarEvalAllocation(t *testing.T) {
	for _, source := range []string{"1 + 2 * (3 + 4);", "-(5 + 6) * 7;", "(8 & 3) | (9 ^ 2);"} {
		t.Run(source, func(t *testing.T) {
			program, err := CompileAnonymous(source)
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			result := &Result{}
			want, err := machine.eval(program.Instructions[0].Expr, result)
			if err != nil {
				t.Fatal(err)
			}
			allocations := testing.AllocsPerRun(100, func() {
				got, err := machine.eval(program.Instructions[0].Expr, result)
				if err != nil || got.Kind != want.Kind || got.Int != want.Int {
					t.Fatalf("evaluation = %v, %v; want %v", got, err, want)
				}
			})
			t.Logf("ROW scalar %s outcome=%d allocations=%.0f", source, want.Int, allocations)
			if allocations != 0 {
				t.Fatalf("scalar evaluation allocated %.0f times; want 0", allocations)
			}
		})
	}
}

func TestRecursiveEvalCallOutcomeRoutes(t *testing.T) {
	const source = `
 List<Integer> items=new List<Integer>{3,5,7};
 Integer cursor=0;
 Integer value=-(items.get(cursor++)+items.get(cursor++))*2;
 System.assertEquals(-16,value);
 System.assertEquals(2,cursor);
 Boolean left=false && items.get(cursor++)==7;
 Boolean right=true || items.get(cursor++)==7;
 System.assertEquals(false,left);
 System.assertEquals(true,right);
 System.assertEquals(2,cursor);
 Integer chosen=true ? items.get(cursor++)+1 : items.get(-1);
 System.assertEquals(8,chosen);
 System.assertEquals(3,cursor);
 Integer fallback=null ?? (items.get(0)+4);
 System.assertEquals(7,fallback);
 List<Integer> row=new List<Integer>{1};
 List<List<Integer>> nested=new List<List<Integer>>{null};
 Integer at=0;
 nested.set(at,row);
 row.set(at,9);
 System.assertEquals(9,nested.get(at).get(at));
 System.assertEquals(9,ROW.get(at));
 List<Integer> missing=null;
 Boolean rejected=false;
 try { missing.get(at); } catch(NullPointerException error){rejected=true;}
 System.assertEquals(true,rejected);
 System.debug('ROW recursive '+value+':'+cursor+':'+chosen+':'+fallback);
 `
	for _, api := range []string{"62.0", "67.0"} {
		for _, tracing := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/traced=%v", api, tracing), func(t *testing.T) {
				program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				machine := New(nil)
				machine.SetTraceEnabled(tracing)
				result, err := machine.Execute(program)
				if err != nil {
					t.Fatal(err)
				}
				if len(result.Debug) != 1 || result.Debug[0] != "ROW recursive -16:3:8:7" {
					t.Fatalf("debug = %q", result.Debug)
				}
				t.Log(result.Debug[0])
			})
		}
	}
}
