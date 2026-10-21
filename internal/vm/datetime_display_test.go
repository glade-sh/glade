package vm

import (
	"fmt"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/storage"
)

func TestDatetimeApexGMTStringConversions(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, zone := range []string{"GMT", "America/Los_Angeles"} {
			for _, route := range []struct {
				name, expression string
			}{
				{"concat-left", "'X'+stamp"},
				{"concat-right", "stamp+'X'"},
				{"valueOf-datetime", "String.valueOf(stamp)"},
				{"valueOf-object", "String.valueOf(boxed)"},
				{"toString", "stamp.toString()"},
			} {
				t.Run(api+"/"+zone+"/"+route.name, func(t *testing.T) {
					want := "2024-02-29 12:34:56"
					// user_timezone the captured row: typed valueOf uses the user
					// zone. Keep the other GMT routes and fractional controls.
					if route.name == "valueOf-datetime" && zone == "America/Los_Angeles" {
						want = "2024-02-29 04:34:56"
					}
					if route.name == "concat-left" {
						want = "X" + want
					} else if route.name == "concat-right" {
						want += "X"
					}
					program, err := CompileAnonymousWithOptions(fmt.Sprintf(`
Long base=1709210096000L;
for(Integer millis : new List<Integer>{0,1,999}) {
    Datetime stamp=Datetime.newInstance(base+millis);
    Object boxed=stamp;
    System.assertEquals('%s',%s);
    System.assertEquals(base+millis,stamp.getTime());
    String fraction=millis==0 ? '000' : (millis==1 ? '001' : '999');
    System.assertEquals('"2024-02-29T12:34:56.'+fraction+'Z"',JSON.serialize(stamp));
}
`, want, route.expression), CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					machine := New(nil)
					machine.SetCurrentUser(storage.Record{ID: "005000000000001", Object: "User", Fields: map[string]storage.Value{
						"TimeZoneSidKey": storage.StringValue(zone),
					}})
					if _, err := machine.Execute(program); err != nil {
						t.Fatal(err)
					}
				})
			}
		}
	}
}

func TestDatetimeNowConcatenationUsesGMTText(t *testing.T) {
	program, err := CompileAnonymous(`System.assertEquals('X2024-02-29 12:34:56','X'+System.now());`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.fakeNow = time.Date(2024, time.February, 29, 12, 34, 56, 123000000, time.UTC)
	machine.SetCurrentUser(storage.Record{ID: "005000000000001", Object: "User", Fields: map[string]storage.Value{
		"TimeZoneSidKey": storage.StringValue("America/Los_Angeles"),
	}})
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestDatetimeStringStaticValueOfUsesGMTText(t *testing.T) {
	for _, fraction := range []string{"000", "001", "999"} {
		t.Run(fraction, func(t *testing.T) {
			value := platformScalar("Datetime", "2024-02-29T12:34:56."+fraction+"Z")
			got, err := stringStatic("String.valueOf", []Value{value})
			if err != nil || got.Kind != ValueString || got.Text != "2024-02-29 12:34:56" {
				t.Fatalf("String.valueOf = %v, %v", got, err)
			}
		})
	}
}
