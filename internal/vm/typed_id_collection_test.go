package vm

import "testing"

// Exact API63 membership controls accepted by Salesforce wave165.
func TestTypedIDCollectionMembership(t *testing.T) {
	for _, tc := range []struct{ name, source string }{
		{"canonicalIdSetUsesEquivalentForms", `  Id shortId='001000000000001';
  Id longId='001000000000001AAA';
  System.assertEquals(true,shortId == longId);
  Set<Id> values=new Set<Id>();
  System.assertEquals(true,values.add(shortId));
  System.assertEquals(true,values.contains(longId));
  System.assertEquals(false,values.add(longId));
  System.assertEquals(1,values.size());
  Set<Id> reverse=new Set<Id>{longId};
  System.assertEquals(true,reverse.contains(shortId));`},
		{"canonicalIdListMembershipUsesEquivalentForms", `  Id shortId='001000000000001';
  Id longId='001000000000001AAA';
  List<Id> shortValues=new List<Id>{shortId};
  List<Id> longValues=new List<Id>{longId};
  System.assertEquals(true,shortValues.contains(longId));
  System.assertEquals(0,shortValues.indexOf(longId));
  System.assertEquals(true,longValues.contains(shortId));
  System.assertEquals(0,longValues.indexOf(shortId));`},
		{"plainStringMembershipKeepsExactFormsDistinct", `  String shortText='001000000000001';
  String longText='001000000000001AAA';
  Set<String> values=new Set<String>{shortText};
  System.assertEquals(false,values.contains(longText));
  System.assertEquals(true,values.add(longText));
  System.assertEquals(2,values.size());
  List<String> shortValues=new List<String>{shortText};
  List<String> longValues=new List<String>{longText};
  System.assertEquals(false,shortValues.contains(longText));
  System.assertEquals(-1,shortValues.indexOf(longText));
  System.assertEquals(false,longValues.contains(shortText));
  System.assertEquals(-1,longValues.indexOf(shortText));`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(tc.source, CompileOptions{APIVersion: "63.0"})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := Execute(program, nil); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestTypedIDCanonicalEqualityAgreesWithMapKeys(t *testing.T) {
	short := String("001000000000001")
	short.Type = "Id"
	long := String("001000000000001AAA")
	long.Type = "Id"
	if !short.Equal(long) || !long.Equal(short) {
		t.Fatal("typed canonical ID forms differ")
	}
	if mapKey(short) != mapKey(long) {
		t.Fatal("equal ID forms have different map keys")
	}
	rawShort := String(short.Text)
	rawLong := String(long.Text)
	if rawShort.Equal(rawLong) {
		t.Fatal("plain String forms compare as IDs")
	}
	if mapKey(rawShort) == mapKey(rawLong) {
		t.Fatal("plain String map keys collapsed")
	}
}
