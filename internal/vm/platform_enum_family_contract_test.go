package vm

import "testing"

func TestExecAPI67PlatformEnumFamilyContracts(t *testing.T) {
	cases := []struct {
		name    string
		program string
		want    string
	}{
		{
			name: "Schema.SoapType",
			program: `
List<String> records = new List<String>();
Integer index = 0;
for (Schema.SoapType value : Schema.SoapType.values()) {
    Schema.SoapType parsed = Schema.SoapType.valueOf(String.valueOf(value).toLowerCase());
    System.assertEquals(index, value.ordinal());
    System.assert(value.equals(parsed));
    records.add(String.valueOf(value) + '|' + value.ordinal() + '|' + value.hashCode());
    index++;
}
String digest = EncodingUtil.convertToHex(Crypto.generateDigest('SHA-256', Blob.valueOf(String.join(records, '\n'))));
System.debug('GLADE_ORACLE_SCHEMA_SOAP_TYPE_CONTRACT=' + index + '|' + digest);
`,
			want: "GLADE_ORACLE_SCHEMA_SOAP_TYPE_CONTRACT=1302|fc0c09ac0a6f06f2363828d716c37d64ccf2b891a0d9fdba2c22bcf0a5e2fb82",
		},
		{
			name: "System.StatusCode",
			program: `
List<String> records = new List<String>();
Integer index = 0;
for (System.StatusCode value : System.StatusCode.values()) {
    System.StatusCode parsed = System.StatusCode.valueOf(String.valueOf(value).toLowerCase());
    System.assertEquals(index, value.ordinal());
    System.assert(value.equals(parsed));
    records.add(String.valueOf(value) + '|' + value.ordinal() + '|' + value.hashCode());
    index++;
}
String digest = EncodingUtil.convertToHex(Crypto.generateDigest('SHA-256', Blob.valueOf(String.join(records, '\n'))));
System.debug('GLADE_ORACLE_SYSTEM_STATUS_CODE_CONTRACT=' + index + '|' + digest);
`,
			want: "GLADE_ORACLE_SYSTEM_STATUS_CODE_CONTRACT=622|5b8a54e66ccedd283e72e9e091c0d860c9ab81068546d4d1268cf90e1eb0b750",
		},
		{
			name: "Metadata.StatusCode",
			program: `
List<String> records = new List<String>();
Integer index = 0;
for (Metadata.StatusCode value : Metadata.StatusCode.values()) {
    Metadata.StatusCode parsed = Metadata.StatusCode.valueOf(String.valueOf(value).toLowerCase());
    System.assertEquals(index, value.ordinal());
    System.assert(value.equals(parsed));
    records.add(String.valueOf(value) + '|' + value.ordinal() + '|' + value.hashCode());
    index++;
}
String digest = EncodingUtil.convertToHex(Crypto.generateDigest('SHA-256', Blob.valueOf(String.join(records, '\n'))));
System.debug('GLADE_ORACLE_METADATA_STATUS_CODE_CONTRACT=' + index + '|' + digest);
`,
			want: "GLADE_ORACLE_METADATA_STATUS_CODE_CONTRACT=513|fe3830197607c1714e70b9ba7391f93edfeab21db6b5f85570622f72e25e57f4",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			program, err := CompileAnonymous(tc.program)
			if err != nil {
				t.Fatal(err)
			}
			result, err := Execute(program, nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Debug) != 1 || result.Debug[0] != tc.want {
				t.Fatalf("debug = %#v, want %#v", result.Debug, []string{tc.want})
			}
		})
	}
}

func TestExecAPI67PlatformEnumValueOfCaseAndErrors(t *testing.T) {
	program, err := CompileAnonymous(`
	System.assert(Schema.SoapType.A_I_APPLICATION.equals(Schema.SoapType.valueOf('a_i_application')));
	System.assert(System.StatusCode.UNKNOWN_EXCEPTION.equals(System.StatusCode.valueOf('unknown_exception')));
	System.assert(Metadata.StatusCode.ALERT_NOTIFICATION_LIMIT_EXCEEDED.equals(Metadata.StatusCode.valueOf('alert_notification_limit_exceeded')));

try {
    Schema.SoapType.valueOf('CB74_MISSING');
    System.assert(false, 'expected Schema.SoapType.valueOf to fail');
} catch (Exception e) {
    System.assertEquals('System.NoSuchElementException', e.getTypeName());
}
try {
    System.StatusCode.valueOf('CB74_MISSING');
    System.assert(false, 'expected System.StatusCode.valueOf to fail');
} catch (Exception e) {
    System.assertEquals('System.NoSuchElementException', e.getTypeName());
}
try {
    Metadata.StatusCode.valueOf('CB74_MISSING');
    System.assert(false, 'expected Metadata.StatusCode.valueOf to fail');
} catch (Exception e) {
    System.assertEquals('System.NoSuchElementException', e.getTypeName());
}
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecAPI67SchemaEnumDeclarationOrder(t *testing.T) {
	program, err := CompileAnonymous(`
List<String> displayTypeNames = new List<String>{
    'STRING', 'BOOLEAN', 'DOUBLE', 'INTEGER', 'PERCENT', 'CURRENCY',
    'DATE', 'DATETIME', 'TIME', 'PICKLIST', 'MULTIPICKLIST',
    'DATACATEGORYGROUPREFERENCE', 'BASE64', 'ID', 'REFERENCE', 'TEXTAREA',
    'PHONE', 'COMBOBOX', 'URL', 'EMAIL', 'ANYTYPE', 'LOCATION',
    'ENCRYPTEDSTRING', 'COMPLEXVALUE', 'ADDRESS', 'SOBJECT', 'LONG', 'JSON',
    'FLOATARRAY', 'TEXTARRAY'
};
List<Schema.DisplayType> displayTypes = Schema.DisplayType.values();
System.assertEquals(displayTypeNames.size(), displayTypes.size());
for (Integer index = 0; index < displayTypeNames.size(); index++) {
    String expected = displayTypeNames.get(index);
    Schema.DisplayType value = displayTypes.get(index);
    Schema.DisplayType exact = Schema.DisplayType.valueOf(expected);
    Schema.DisplayType lower = Schema.DisplayType.valueOf(expected.toLowerCase());
    System.assertEquals(expected, value.name());
    System.assertEquals(expected, value.toString());
    System.assertEquals(expected, String.valueOf(value));
    System.assertEquals(index, value.ordinal());
    System.assert(value.equals(exact));
    System.assert(value.equals(lower));
    System.assert(exact.equals(lower));
    Integer hash = value.hashCode();
    System.assertEquals(hash, value.hashCode());
    System.assertEquals(hash, exact.hashCode());
    System.assertEquals(hash, lower.hashCode());
}

List<String> optionNames = new List<String>{'DEFAULT', 'FULL', 'DEFERRED'};
List<Schema.SObjectDescribeOptions> qualifiedOptions = Schema.SObjectDescribeOptions.values();
List<SObjectDescribeOptions> unqualifiedOptions = SObjectDescribeOptions.values();
System.assertEquals(optionNames.size(), qualifiedOptions.size());
System.assertEquals(optionNames.size(), unqualifiedOptions.size());
for (Integer index = 0; index < optionNames.size(); index++) {
    String expected = optionNames.get(index);
    Schema.SObjectDescribeOptions qualified = qualifiedOptions.get(index);
    Schema.SObjectDescribeOptions qualifiedExact = Schema.SObjectDescribeOptions.valueOf(expected);
    Schema.SObjectDescribeOptions qualifiedLower = Schema.SObjectDescribeOptions.valueOf(expected.toLowerCase());
    System.assertEquals(expected, qualified.name(), 'qualified name');
    System.assertEquals(expected, qualified.toString(), 'qualified toString');
    System.assertEquals(expected, String.valueOf(qualified), 'qualified String.valueOf');
    System.assertEquals(index, qualified.ordinal());
    System.assert(qualified.equals(qualifiedExact));
    System.assert(qualified.equals(qualifiedLower));
    System.assert(qualifiedExact.equals(qualifiedLower));
    Integer qualifiedHash = qualified.hashCode();
    System.assertEquals(qualifiedHash, qualified.hashCode());
    System.assertEquals(qualifiedHash, qualifiedExact.hashCode());
    System.assertEquals(qualifiedHash, qualifiedLower.hashCode());

    SObjectDescribeOptions unqualified = unqualifiedOptions.get(index);
    SObjectDescribeOptions unqualifiedExact = SObjectDescribeOptions.valueOf(expected);
    SObjectDescribeOptions unqualifiedLower = SObjectDescribeOptions.valueOf(expected.toLowerCase());
    System.assertEquals(expected, unqualified.name(), 'unqualified name');
    System.assertEquals(expected, unqualified.toString(), 'unqualified toString');
    System.assertEquals(index, unqualified.ordinal());
    System.assert(unqualified.equals(unqualifiedExact));
    System.assert(unqualified.equals(unqualifiedLower));
    System.assert(unqualifiedExact.equals(unqualifiedLower));
    Integer unqualifiedHash = unqualified.hashCode();
    System.assertEquals(unqualifiedHash, unqualified.hashCode());
    System.assertEquals(unqualifiedHash, unqualifiedExact.hashCode());
    System.assertEquals(unqualifiedHash, unqualifiedLower.hashCode());
}
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecUserEnumHelpersRetainDeclarationOrder(t *testing.T) {
	program, err := CompileAnonymous(`
List<CB74UserEnum.Mode> values = CB74UserEnum.Mode.values();
System.assertEquals(3, values.size());
System.assertEquals('SECOND', String.valueOf(values.get(0)));
System.assertEquals('FIRST', String.valueOf(values.get(1)));
System.assertEquals('THIRD', String.valueOf(values.get(2)));
System.assertEquals(0, values.get(0).ordinal());
System.assertEquals(1, values.get(1).ordinal());
System.assertEquals(2, values.get(2).ordinal());
CB74UserEnum.Mode parsed = CB74UserEnum.Mode.valueOf('first');
System.assert(values.get(1).equals(parsed));
Integer storedHash = values.get(1).hashCode();
System.assertEquals(storedHash, values.get(1).hashCode());
Integer parsedHash = parsed.hashCode();
System.assertEquals(parsedHash, parsed.hashCode());
try {
    CB74UserEnum.Mode.valueOf('CB74_MISSING');
    System.assert(false, 'expected CB74UserEnum.Mode.valueOf to fail');
} catch (Exception e) {
    System.assertEquals('System.NoSuchElementException', e.getTypeName());
}
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name:       "CB74UserEnum.Mode",
		EnumValues: []string{"SECOND", "FIRST", "THIRD"},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}
