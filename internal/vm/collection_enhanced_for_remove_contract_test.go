package vm

import "testing"

// API67 ordinary nonzero-Ref List/Set enhanced-for removals: retained guide
// apex-guide/langCon_apex_collections_iterating.md SHA256
// 0e3c2ebc90e51d62a9cb4a4532ed8876f35b857f1d01c721b5a1f8e751abe648,
// lines 3-6 forbid direct removal during iteration; lines 17-21 and 28-30
// admit staged removal after traversal. List.remove semantics are retained in
// apex/apex_methods_system_list.md SHA256
// 195f91a116ecd28fca7dd8a62dface15c6ed55e2a44bbff485cd74b88b2ca6a3,
// lines 677-704; Set.remove semantics are retained in
// apex/apex_methods_system_set.md SHA256
// 2ad26377419fb375911ecf43a2e7e7b9b10b27db0170e6f9f917a7dd4feef723,
// lines 410-428. Four exact guard3274 receipt bodies (SHA256
// cc34d411f7bcf067a1af3b4e0360e7300513138647356b18fa753ae5f5a36a38)
// establish two execution-error-existence predicates and seven after-loop
// assertions. No exception class, message, catchability or timing is asserted.
// Ref0, bulk mutations, clear, Map and iterator-creation locks are unqualified.
func TestExecCollectionEnhancedForRejectsStructuralRemoveAPI67(t *testing.T) {
	cases := []struct {
		name      string
		source    string
		wantError bool
	}{
		{name: "ListActiveRemove", source: `List<Integer> values=new List<Integer>{1,2,3};Boolean first=true;for(Integer loopValue:values){if(first){first=false;values.remove(0);}}`, wantError: true},
		{name: "SetActiveRemove", source: `Set<Integer> values=new Set<Integer>{1,2,3};Boolean first=true;for(Integer loopValue:values){if(first){first=false;values.remove(loopValue);}}`, wantError: true},
		{name: "ListAfterTraversalRemove", source: `List<Integer> values=new List<Integer>{1,2,3};Integer visited=0;for(Integer loopValue:values){visited++;}System.assertEquals(3,visited);System.assertEquals(1,values.remove(0));System.assertEquals(2,values.size());`, wantError: false},
		{name: "SetAfterTraversalRemove", source: `Set<Integer> values=new Set<Integer>{1,2,3};Integer visited=0;Integer selected=null;for(Integer loopValue:values){visited++;if(selected==null){selected=loopValue;}}System.assertEquals(3,visited);System.assertEquals(true,values.remove(selected));System.assertEquals(2,values.size());System.assertEquals(false,values.contains(selected));`, wantError: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(tc.source, CompileOptions{APIVersion: "67.0"})
			if err != nil {
				t.Fatal(err)
			}
			if program.APIVersion != "67.0" {
				t.Fatalf("compiled API version = %q, want 67.0", program.APIVersion)
			}
			_, err = New(nil).Execute(program)
			if (err != nil) != tc.wantError {
				t.Fatalf("execution error = %v, want error existence %v", err, tc.wantError)
			}
		})
	}
}
