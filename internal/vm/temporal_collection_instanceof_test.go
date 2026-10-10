package vm

import "testing"

func TestExecTemporalCollectionInstanceOfDoesNotUseConversion(t *testing.T) {
	program, err := CompileAnonymousWithOptions(`
Datetime stamp=Datetime.newInstanceGmt(2020,6,8,0,2,3);
Object dates=new List<Datetime>{stamp};
System.assertEquals(false,dates instanceof List<Date>);
System.assertEquals(true,dates instanceof List<Datetime>);
Object uniqueDates=new Set<Datetime>{stamp};
System.assertEquals(false,uniqueDates instanceof Set<Date>);
System.assertEquals(true,uniqueDates instanceof Set<Datetime>);
System.assertEquals(stamp,((List<Datetime>)dates)[0]);
Object dateControl=new List<Date>{Date.newInstance(2020,6,8)};
System.assertEquals(true,dateControl instanceof List<Date>);
Date source=Date.newInstance(2020,6,8);
Datetime converted=source;
System.assertEquals(source,converted.date());
`, CompileOptions{APIVersion: "63.0"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}
