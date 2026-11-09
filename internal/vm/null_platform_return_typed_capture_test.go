package vm

import (
	"fmt"
	"testing"
)

// Native rows (testdata/typed_null_platform_returns/org.tsv, 62.0 and 67.0): these
// platform calls returns null typed by its declared return, so the overload
// for that type is chosen, the value equals null, 'x' + String.valueOf(v) is
// 'xnull' and an Object holding it is not an instance of the declared type.
func TestTypedNullCapturedPlatformReturns(t *testing.T) {
	const check = ` System.assertEquals(true, v == null); System.assertEquals('valueOf=null', 'valueOf=' + String.valueOf(v)); System.assertEquals(false, v instanceof %s);`
	for _, tc := range []struct {
		name, setup, call, want string
		test                    bool
	}{
		// SnNull site.direct and site.two, anonymous and isTest.
		{name: "Site.changePassword anonymous", call: "Site.changePassword('Abcdef123!', 'Abcdef123!', 'Old12345!')", want: "PageReference"},
		{name: "Site.changePassword two args anonymous", call: "Site.changePassword('Abcdef123!', 'Abcdef123!')", want: "PageReference"},
		{name: "Site.changePassword isTest", call: "Site.changePassword('Abcdef123!', 'Abcdef123!', 'Old12345!')", want: "PageReference", test: true},
		// SnNull session.direct and session.profile, anonymous.
		{name: "getRequiredSessionLevelForProfile null", call: "Auth.SessionManagement.getRequiredSessionLevelForProfile(null)", want: "Auth.SessionLevel"},
		{name: "getRequiredSessionLevelForProfile profile", call: "Auth.SessionManagement.getRequiredSessionLevelForProfile(UserInfo.getProfileId())", want: "Auth.SessionLevel"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			machine := New(nil)
			if tc.test {
				machine.EnableTestContext()
			}
			registerTypedNullOverloads(t, machine)
			source := "String r = Q.f(" + tc.call + "); System.assertEquals('" + tc.want + "', r); Object v = " + tc.call + ";" +
				fmt.Sprintf(check, tc.want)
			program, err := CompileAnonymous(source)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// SnQueue rows: inside a Queueable enqueued without a delay the value is null
// and picks the Integer overload; with a delay it is that Integer.
func TestTypedNullQueueableDelayWithoutDelay(t *testing.T) {
	for _, tc := range []struct{ name, enqueue, want string }{
		{"no delay", "System.enqueueJob(new QueueWorker());", "Integer:null"},
		{"delay 1", "System.enqueueJob(new QueueWorker(), 1);", "Integer:1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			program, err := CompileAnonymous("Test.startTest(); " + tc.enqueue + " Test.stopTest(); System.assertEquals('" + tc.want + "', QueueWorker.seen);")
			if err != nil {
				t.Fatal(err)
			}
			queueProgram, err := CompileAnonymous(`Integer d = AsyncInfo.getMinimumQueueableDelayInMinutes();
QueueWorker.seen = Q.f(AsyncInfo.getMinimumQueueableDelayInMinutes()) + ':' + String.valueOf(d);`)
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			machine.EnableTestContext()
			registerTypedNullOverloads(t, machine)
			if err := machine.RegisterClass(Class{
				Name:         "QueueWorker",
				Interfaces:   []string{"Queueable"},
				StaticFields: map[string]Field{"seen": {Name: "seen", Type: "String"}},
				Methods: map[string]Method{
					"execute": {
						Name:       "QueueWorker.execute",
						ClassName:  "QueueWorker",
						ReturnType: "void",
						Params:     []Param{{Name: "context", Type: "QueueableContext"}},
						Program:    queueProgram,
					},
				},
			}); err != nil {
				t.Fatal(err)
			}
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func registerTypedNullOverloads(t *testing.T, machine *VM) {
	t.Helper()
	if err := machine.RegisterClass(Class{Name: "Q"}); err != nil {
		t.Fatal(err)
	}
	for _, spec := range []string{"String", "Integer", "PageReference", "Auth.SessionLevel"} {
		body, err := CompileAnonymous("return '" + spec + "';")
		if err != nil {
			t.Fatal(err)
		}
		if err := machine.RegisterMethod(Method{Name: "Q.f", ClassName: "Q", IsStatic: true, ReturnType: "String", Params: []Param{{Name: "value", Type: spec}}, Program: body}); err != nil {
			t.Fatal(err)
		}
	}
}
