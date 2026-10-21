package vm

import "testing"

// Scalar Id text is captured in id_uuid_version I005/I011. PENDING-ROWS:
// the nested Map/Set projection needs independent native formatting rows.
func TestIDSetTextInsideObjectMap(t *testing.T) {
	program, err := CompileAnonymous(`Id recordId='001000000000001'; Set<Id> ids=new Set<Id>{recordId}; Map<String,Object> bag=new Map<String,Object>{'slot'=>ids}; System.assertEquals('{slot={001000000000001AAA}}',String.valueOf(bag)); Map<String,Object> control=new Map<String,Object>{'slot'=>new Set<String>{'001000000000001'}};System.assertEquals('{slot={001000000000001}}',String.valueOf(control));`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}
