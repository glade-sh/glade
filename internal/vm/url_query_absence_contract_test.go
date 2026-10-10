package vm

import "testing"

// the native compatibility contract targets API67 instances under the API41+ URI profile.
// Public source: apex_methods_system_url.md, lines 438–452.
// Source SHA256: 4603656c0a4296f10e64939ee225286de77635bf06d8692348e40568e7b43b6b.
// Summer26 catalog: documents[2225].members[14], apex:System.URL.getQuery().
// Catalog SHA256: 9d96c8f2472acb64d9d2860ab22ebb1e526edccf137b659c2bca441b1ce86a7b.
func TestExecURLQueryAbsenceAPI67(t *testing.T) {
	for _, tc := range []struct {
		name   string
		source string
	}{
		{
			name: "noQuery",
			source: `
URL value = new URL('https://example.test/path');
System.assertEquals(null, value.getQuery());
`,
		},
		{
			name: "fragmentOnly",
			source: `
URL value = new URL('https://example.test/path#anchor');
System.assertEquals(null, value.getQuery());
`,
		},
		{
			name: "presentQuery",
			source: `
URL value = new URL('https://example.test/path?id=001');
System.assert(value.getQuery().equals('id=001'));
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
