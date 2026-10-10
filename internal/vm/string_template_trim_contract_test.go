package vm

import "testing"

// API67 compiled callers and helper body; manually registered named class.
func TestExecStringTemplateAndTrimContractAPI67(t *testing.T) {
	const helperSource = "return 'override-sentinel';"
	cases := []struct {
		name       string
		usesHelper bool
		source     string
	}{
		{
			name: "templateOverride",
			usesHelper: true,
			source: `GladeTemplateValue value = new GladeTemplateValue();
Map<String,Object> values = new Map<String,Object>{'value' => value};
String actual = '${value}'.template(values);
System.assertEquals('override-sentinel', actual);
`,
		},
		{
			name: "directOverrideControl",
			usesHelper: true,
			source: `GladeTemplateValue value = new GladeTemplateValue();
String actual = value.toString();
System.assertEquals('override-sentinel', actual);
`,
		},
		{
			name: "templateStringControl",
			usesHelper: false,
			source: `Map<String,Object> values = new Map<String,Object>{'value' => 'plain'};
String actual = '${value}'.template(values);
System.assertEquals('plain', actual);
`,
		},
		{
			name: "templateIntegerControl",
			usesHelper: false,
			source: `Map<String,Object> values = new Map<String,Object>{'value' => 22};
String actual = '${value}'.template(values);
System.assertEquals('22', actual);
`,
		},
		{
			name: "templateEscapedControl",
			usesHelper: false,
			source: `Map<String,Object> values = new Map<String,Object>{'value' => 'plain'};
String actual = '$${value}'.template(values);
System.assertEquals('${value}', actual);
`,
		},
		{
			name: "templateMissingKeyControl",
			usesHelper: false,
			source: `Boolean caught = false;
try {
    '${missing}'.template(new Map<String,Object>());
} catch (System.StringException e) {
    caught = true;
}
System.assertEquals(true, caught);
`,
		},
		{
			name: "trimBoundary",
			usesHelper: false,
			source: `String ctrl = String.fromCharArray(new List<Integer>{1});
String text = ctrl + 'x' + ctrl;
String actual = text.trim();
System.assertEquals('x', actual);
`,
		},
		{
			name: "trimInteriorControl",
			usesHelper: false,
			source: `String ctrl = String.fromCharArray(new List<Integer>{1});
String text = 'x' + ctrl + 'y';
String actual = text.trim();
System.assertEquals(text, actual);
`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			options := CompileOptions{APIVersion: "67.0"}
			machine := New(nil)
			if tc.usesHelper {
				helperProgram, helperErr := CompileAnonymousWithOptions(helperSource, options)
				if helperErr != nil {
					t.Fatalf("step=helper-compile case=%s api=67.0 source=%q compileErr=%v", tc.name, helperSource, helperErr)
				}
				if helperProgram.APIVersion != "67.0" {
					t.Fatalf("step=helper-program-api-guard case=%s api=67.0 source=%q got=%q want=67.0 error=compiled-api-mismatch", tc.name, helperSource, helperProgram.APIVersion)
				}
				if registerErr := machine.RegisterClass(Class{
					Name:       "GladeTemplateValue",
					Namespace:  "",
					APIVersion: "67.0",
					Access:     "public",
					Methods: map[string]Method{
						"toString": {
							Name:       "GladeTemplateValue.toString",
							ClassName:  "GladeTemplateValue",
							ReturnType: "String",
							APIVersion: "67.0",
							Access:     "public",
							IsStatic:   false,
							Program:    helperProgram,
						},
					},
				}); registerErr != nil {
					t.Fatalf("step=helper-register case=%s api=67.0 registerErr=%v", tc.name, registerErr)
				}
			}
			program, compileErr := CompileAnonymousWithOptions(tc.source, options)
			if compileErr != nil {
				t.Fatalf("step=compile case=%s api=67.0 source=%q compileErr=%v", tc.name, tc.source, compileErr)
			}
			if program.APIVersion != "67.0" {
				t.Fatalf("step=program-api-guard case=%s api=67.0 source=%q got=%q want=67.0 error=compiled-api-mismatch", tc.name, tc.source, program.APIVersion)
			}
			if _, executeErr := machine.Execute(program); executeErr != nil {
				t.Fatalf("step=execute case=%s api=67.0 source=%q executeErr=%v", tc.name, tc.source, executeErr)
			}
		})
	}
}
