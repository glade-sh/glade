package vm

import (
	"fmt"
	"testing"
)

// Owned API63 named observations accept both short-year inputs, and preserve
// the padded JSON serialization and equality of the corresponding dates.
// Historical calendar accessors are deliberately outside this regression.
func TestJSONDeserializeShortYearDateScalar(t *testing.T) {
	for _, pair := range []struct{ short, padded string }{
		{"1-12-25", "0001-12-25"},
		{"33-04-18", "0033-04-18"},
	} {
		t.Run(pair.short, func(t *testing.T) {
			source := fmt.Sprintf(`
Date shortValue=(Date)JSON.deserialize('"%s"',Date.class);
Date paddedValue=(Date)JSON.deserialize('"%s"',Date.class);
System.assertEquals(paddedValue,shortValue);
System.assertEquals('"%s"',JSON.serialize(shortValue));
`, pair.short, pair.padded, pair.padded)
			program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: "63.0"})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := Execute(program, nil); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestJSONDeserializeShortYearDateReadonlyNested(t *testing.T) {
	machine := New(nil)
	for _, class := range []Class{
		{Name: "DateEnvelope", APIVersion: "63.0", Fields: map[string]Field{
			"entries": {Name: "entries", Type: "Map<String,List<DateEnvelope.Window>>", Property: true, HasGetter: true},
		}},
		{Name: "DateEnvelope.Window", APIVersion: "63.0", Fields: map[string]Field{
			"starts": {Name: "starts", Type: "Date", Property: true, HasGetter: true},
			"ends":   {Name: "ends", Type: "Date", Property: true, HasGetter: true},
		}},
	} {
		if err := machine.RegisterClass(class); err != nil {
			t.Fatal(err)
		}
	}
	program, err := CompileAnonymousWithOptions(`
DateEnvelope shortValue=(DateEnvelope)JSON.deserialize('{"entries":{"owned":[{"starts":"1-12-25","ends":"33-04-18"}]}}',DateEnvelope.class);
DateEnvelope paddedValue=(DateEnvelope)JSON.deserialize('{"entries":{"owned":[{"starts":"0001-12-25","ends":"0033-04-18"}]}}',DateEnvelope.class);
DateEnvelope.Window shortWindow=shortValue.entries.get('owned')[0];
DateEnvelope.Window paddedWindow=paddedValue.entries.get('owned')[0];
System.assertEquals(paddedWindow.starts,shortWindow.starts);
System.assertEquals(paddedWindow.ends,shortWindow.ends);
System.assertEquals('"0001-12-25"',JSON.serialize(shortWindow.starts));
System.assertEquals('"0033-04-18"',JSON.serialize(shortWindow.ends));
`, CompileOptions{APIVersion: "63.0"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestJSONDeserializeShortYearDateKeepsConversionBoundaries(t *testing.T) {
	program, err := CompileAnonymousWithOptions(`
Date ordinary=(Date)JSON.deserialize('"2004-02-29"',Date.class);
System.assertEquals('"2004-02-29"',JSON.serialize(ordinary));
System.assertEquals(Date.newInstance(2004,2,29),ordinary);
Boolean invalidCaught=false;
try { JSON.deserialize('"2004-02-30"',Date.class); }
catch(JSONException failure) { invalidCaught=true; }
System.assert(invalidCaught);
Boolean numericCaught=false;
try { JSON.deserialize('12',Date.class); }
catch(JSONException failure) { numericCaught=true; }
System.assert(numericCaught);
// Preservation control: this JSON fix must not widen Date.valueOf parsing.
Boolean valueOfCaught=false;
try { Date.valueOf('1-12-25'); }
catch(TypeException failure) { valueOfCaught=true; }
System.assert(valueOfCaught);
System.assertEquals('2004-02-29',String.valueOf(Date.valueOf('2004-02-29')));
`, CompileOptions{APIVersion: "63.0"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}
