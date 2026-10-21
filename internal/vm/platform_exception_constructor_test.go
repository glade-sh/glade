package vm

import "testing"

// The platform-stub result is captured in cache_exception_constructor. The
// other cases preserve explicit source and executable-constructor precedence.
func TestMatchedPlatformExceptionConstructorMessages(t *testing.T) {
	for _, tc := range []struct {
		name, class, want string
		platform, body    bool
	}{
		{name: "sourceEmpty", class: "OwnedException", want: "Script-thrown exception"},
		{name: "sourceBody", class: "OwnedException", body: true, want: "constructor body"},
		{name: "platformBody", class: "cache.CacheException", platform: true, body: true, want: "constructor body"},
		{name: "platformStub", class: "cache.CacheException", platform: true, want: "argument message"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			constructor := Method{
				Name: tc.class + ".<init>", ClassName: tc.class, IsConstructor: true,
				ReturnType: "void", Access: "global", Params: []Param{{Name: "input", Type: "String"}},
			}
			if tc.body {
				var err error
				constructor.Program, err = CompileAnonymous(`this.setMessage('constructor body');`)
				if err != nil {
					t.Fatal(err)
				}
			}
			machine := New(nil)
			parent := "Exception"
			if tc.platform {
				parent = "Object" // Standard stub superclass metadata.
			}
			if err := machine.RegisterClass(Class{
				Name: tc.class, PlatformType: tc.platform, SuperClass: parent, Access: "global",
				Constructors: []Method{constructor},
			}); err != nil {
				t.Fatal(err)
			}
			program, err := CompileAnonymous(tc.class + ` value = new ` + tc.class + `('argument message'); System.assertEquals('` + tc.want + `',value.getMessage());`)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
