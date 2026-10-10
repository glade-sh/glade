package vm

import "testing"

func TestExecJSONParserGetTextTokenAbsenceAPI67(t *testing.T) {
	for _, tc := range []struct {
		name   string
		source string
	}{
		{
			name: "fresh",
			source: `
JSONParser parser = JSON.createParser('"hello"');
System.assertEquals(null, parser.getText());
`,
		},
		{
			name: "cleared",
			source: `
JSONParser parser = JSON.createParser('"hello"');
parser.nextToken();
parser.clearCurrentToken();
System.assertEquals(null, parser.getText());
`,
		},
		{
			name: "eof",
			source: `
JSONParser parser = JSON.createParser('"hello"');
parser.nextToken();
parser.nextToken();
System.assertEquals(null, parser.getText());
`,
		},
		{
			name: "literalText",
			source: `
JSONParser parser = JSON.createParser('"hello"');
parser.nextToken();
System.assert(parser.getText().equals('hello'));
`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(tc.source, CompileOptions{APIVersion: "67.0"})
			if err != nil {
				t.Fatal(err)
			}
			if program.APIVersion != "67.0" {
				t.Fatalf("compiled API version = %q, want 67.0", program.APIVersion)
			}
			if _, err := Execute(program, nil); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestExecJSONParserIntegerDomainAPI67(t *testing.T) {
	for _, tc := range []struct {
		name          string
		longSource    string
		integerSource string
		reject        bool
	}{
		{
			name: "minimum",
			longSource: `
JSONParser parser = JSON.createParser('-2147483648');
parser.nextToken();
System.assertEquals(-2147483648L, parser.getLongValue());
`,
			integerSource: `
JSONParser parser = JSON.createParser('-2147483648');
parser.nextToken();
System.assertEquals(-2147483647 - 1, parser.getIntegerValue());
`,
		},
		{
			name: "maximum",
			longSource: `
JSONParser parser = JSON.createParser('2147483647');
parser.nextToken();
System.assertEquals(2147483647L, parser.getLongValue());
`,
			integerSource: `
JSONParser parser = JSON.createParser('2147483647');
parser.nextToken();
System.assertEquals(2147483647, parser.getIntegerValue());
`,
		},
		{
			name: "belowMinimum",
			longSource: `
JSONParser parser = JSON.createParser('-2147483649');
parser.nextToken();
System.assertEquals(-2147483649L, parser.getLongValue());
`,
			integerSource: `
JSONParser parser = JSON.createParser('-2147483649');
parser.nextToken();
parser.getIntegerValue();
`,
			reject: true,
		},
		{
			name: "aboveMaximum",
			longSource: `
JSONParser parser = JSON.createParser('2147483648');
parser.nextToken();
System.assertEquals(2147483648L, parser.getLongValue());
`,
			integerSource: `
JSONParser parser = JSON.createParser('2147483648');
parser.nextToken();
parser.getIntegerValue();
`,
			reject: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			longProgram, err := CompileAnonymousWithOptions(tc.longSource, CompileOptions{APIVersion: "67.0"})
			if err != nil {
				t.Fatal(err)
			}
			if longProgram.APIVersion != "67.0" {
				t.Fatalf("compiled Long control API version = %q, want 67.0", longProgram.APIVersion)
			}
			if _, err := Execute(longProgram, nil); err != nil {
				t.Fatalf("independent Long control failed: %v", err)
			}

			integerProgram, err := CompileAnonymousWithOptions(tc.integerSource, CompileOptions{APIVersion: "67.0"})
			if err != nil {
				t.Fatal(err)
			}
			if integerProgram.APIVersion != "67.0" {
				t.Fatalf("compiled Integer API version = %q, want 67.0", integerProgram.APIVersion)
			}
			_, err = Execute(integerProgram, nil)
			if tc.reject {
				if err == nil {
					t.Fatal("getIntegerValue accepted a value outside the Integer domain")
				}
			} else if err != nil {
				t.Fatal(err)
			}
		})
	}
}
