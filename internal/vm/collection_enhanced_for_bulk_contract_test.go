package vm

import "testing"

// API67 ordinary nonzero-Ref List-argument bulk/clear facet: retained guide
// apex-guide/langCon_apex_collections_iterating.md SHA256
// 0e3c2ebc90e51d62a9cb4a4532ed8876f35b857f1d01c721b5a1f8e751abe648,
// lines 3-6 prohibit structural additions/removals during iteration; deferred
// clauses admit mutation after traversal. List guide SHA256
// 195f91a116ecd28fca7dd8a62dface15c6ed55e2a44bbff485cd74b88b2ca6a3
// and Set guide SHA256
// 2ad26377419fb375911ecf43a2e7e7b9b10b27db0170e6f9f917a7dd4feef723
// bind the method contracts. Retained API67 catalog SHA256
// 9d96c8f2472acb64d9d2860ab22ebb1e526edccf137b659c2bca441b1ce86a7b
// identifies List addAll/clear at /documents/2205/members/5,7 and Set
// addAll/removeAll/retainAll/clear at /documents/2215/members/4,15,17,6.
// These eight exact guard16272 receipt bodies (SHA256
// d9fc09c25f5e405a8a07df0ba7a2ad558a3d4113ee8bdc30a57193d9d787058a)
// establish six execution-error-existence predicates and eleven after-loop
// assertions. No exception class, message, catchability or timing is asserted.
// No-op, Ref0, Iterator, Map, other overloads and whole-family/native parity
// remain unqualified.
func TestExecCollectionEnhancedForRejectsStructuralBulkMutationsAPI67(t *testing.T) {
	cases := []struct {
		name      string
		source    string
		wantError bool
	}{
		{name: "ListAddAll", source: `List<Integer> values=new List<Integer>{1,2,3};Boolean first=true;for(Integer value:values){if(first){first=false;values.addAll(new List<Integer>{4});}}`, wantError: true},
		{name: "SetAddAll", source: `Set<Integer> values=new Set<Integer>{1,2,3};Boolean first=true;for(Integer value:values){if(first){first=false;values.addAll(new List<Integer>{4});}}`, wantError: true},
		{name: "SetRemoveAll", source: `Set<Integer> values=new Set<Integer>{1,2,3};Boolean first=true;for(Integer value:values){if(first){first=false;values.removeAll(new List<Integer>{1});}}`, wantError: true},
		{name: "SetRetainAll", source: `Set<Integer> values=new Set<Integer>{1,2,3};Boolean first=true;for(Integer value:values){if(first){first=false;values.retainAll(new List<Integer>{1});}}`, wantError: true},
		{name: "ListClear", source: `List<Integer> values=new List<Integer>{1,2,3};Boolean first=true;for(Integer value:values){if(first){first=false;values.clear();}}`, wantError: true},
		{name: "SetClear", source: `Set<Integer> values=new Set<Integer>{1,2,3};Boolean first=true;for(Integer value:values){if(first){first=false;values.clear();}}`, wantError: true},
		{name: "ListBulkAfterTraversal", source: `List<Integer> values=new List<Integer>{1,2,3};Integer visits=0;Integer total=0;for(Integer value:values){visits++;total+=value;}System.assertEquals(3,visits);System.assertEquals(6,total);values.addAll(new List<Integer>{4});System.assertEquals(4,values.size());values.clear();System.assertEquals(0,values.size());`, wantError: false},
		{name: "SetBulkAfterTraversal", source: `Set<Integer> values=new Set<Integer>{1,2,3};Integer visits=0;Integer total=0;for(Integer value:values){visits++;total+=value;}System.assertEquals(3,visits);System.assertEquals(6,total);values.addAll(new List<Integer>{4});System.assertEquals(4,values.size());values.removeAll(new List<Integer>{1});System.assertEquals(3,values.size());values.retainAll(new List<Integer>{2});System.assertEquals(1,values.size());System.assert(values.contains(2));values.clear();System.assertEquals(0,values.size());`, wantError: false},
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
