package vm

import "testing"

// Native rows: ../apextest/testdata/conformance/json_system_address/native
// (API 62.0 and 67.0 agree).
func TestJSONDeserializeSystemAddressMatchesNativeRows(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(`
Address a = (Address) JSON.deserialize('{"city":"San Francisco","country":"USA","postalCode":"94103","state":"CA","street":"123 Market St"}', Address.class);
System.assertEquals('San Francisco', a.city);
System.assertEquals('USA', a.country);
System.assertEquals('94103', a.postalCode);
System.assertEquals('CA', a.state);
System.assertEquals('123 Market St', a.street);
System.assertEquals(null, a.countryCode);
System.assertEquals(null, a.latitude);

System.Address b = (System.Address) JSON.deserialize('{"city":"A","stateCode":"CA","countryCode":"US","latitude":37.5,"longitude":-122.25,"geocodeAccuracy":"Street"}', System.Address.class);
System.assertEquals('A', b.city);
System.assertEquals('CA', b.stateCode);
System.assertEquals('US', b.countryCode);
System.assertEquals(37.5, b.latitude);
System.assertEquals(-122.25, b.longitude);
System.assertEquals('Street', b.geocodeAccuracy);
System.assertEquals(null, b.state);

Address c = (Address) JSON.deserialize('{}', Address.class);
System.assertEquals(null, c.city);

Address d = (Address) JSON.deserialize('{"city":1}', Address.class);
System.assertEquals('1', d.city);

System.assertEquals(null, (Address) JSON.deserialize('null', Address.class));

try {
  JSON.deserialize('{"city":"A","bogus":"x"}', Address.class);
  System.assert(false, 'unknown field accepted');
} catch (JSONException e) {
  System.assertEquals('Unrecognized field "bogus" at [line:1, column:22]', e.getMessage());
}

String[] bad = new String[]{'{"bogus":"x"}', '{"bogus": 12345}', '{"city":"A","bogus":true,"z":1}', '{"bogus":null}', '{"bogus":{"a":1}}'};
String[] badMsg = new String[]{'column:11', 'column:16', 'column:25', 'column:14', 'column:11'};
for (Integer i = 0; i < bad.size(); i++) {
  try {
    JSON.deserialize(bad[i], Address.class);
    System.assert(false, 'unknown field accepted');
  } catch (JSONException ex) {
    System.assert(ex.getMessage().endsWith(badMsg[i] + ']'), ex.getMessage());
  }
}
try {
  JSON.deserialize('{"city":"A",\n "bogus":"x"}', Address.class);
  System.assert(false, 'unknown field accepted');
} catch (JSONException ex) {
  System.assertEquals('Unrecognized field "bogus" at [line:2, column:11]', ex.getMessage());
}

Address e2 = (Address) JSON.deserialize('{"city":"X","street":"S"}', Address.class);
System.assertEquals('{"city":"X","country":null,"geocodeAccuracy":null,"latitude":null,"longitude":null,"postalCode":null,"state":null,"street":"S"}', JSON.serialize(e2));
`, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			machine.EnableTestContext()
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
