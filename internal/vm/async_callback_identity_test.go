package vm

import "testing"

func TestAsyncContextJobIDNativeRepresentation(t *testing.T) {
	for _, tc := range []struct {
		route, getter, want string
		context             Value
	}{
		{"Queueable", "getJobId", "707000000000001AAA", asyncContext("QueueableContext", "707000000000001")},
		{"BatchStartExecuteFinish", "getJobId", "707000000000001AAA", asyncContext("Database.BatchableContext", "707000000000001")},
		{"Scheduled", "getTriggerId", "08e000000000001AAA", schedulableContext("707000000000001")},
		{"Finalizer", "getAsyncApexJobId", "707000000000001AAA", finalizerContext("707000000000001", nil)},
	} {
		t.Run(tc.route, func(t *testing.T) {
			value, _, _, handled, err := New(nil).callPlatformObjectMember(tc.context, tc.getter, nil, &Result{})
			if err != nil || !handled {
				t.Fatalf("context getter: handled=%t error=%v", handled, err)
			}
			text, err := platformScalarText(value, "Id")
			if err != nil || text != tc.want {
				t.Fatalf("native context Id=%s; Glade type=%s raw=%q (%v)", tc.want, value.Type, text, err)
			}
		})
	}
}

func TestAsyncStaticCallbacksDirectDispatchNative(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, kind := range []string{"Queueable", "ScheduledApex", "BatchApex", "ScheduledBatch"} {
			for _, static := range []bool{false, true} {
				mode := "instance"
				if static {
					mode = "static"
				}
				t.Run(api+"/"+kind+"/"+mode, func(t *testing.T) {
					machine := New(nil)
					class := Class{Name: "CallbackWorker", APIVersion: api, Methods: map[string]Method{}}
					add := func(name, returns, body string, params ...Param) {
						t.Helper()
						program, err := CompileAnonymousWithOptions(body, CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						class.Methods[name] = Method{Name: class.Name + "." + name, ClassName: class.Name, APIVersion: api, IsStatic: static, ReturnType: returns, Params: params, Program: program}
					}
					want := 1
					switch kind {
					case "Queueable":
						class.Interfaces = []string{"Queueable"}
						add("execute", "void", "System.debug('execute');", Param{Name: "context", Type: "QueueableContext"})
					case "ScheduledApex":
						class.Interfaces = []string{"Schedulable"}
						add("execute", "void", "System.debug('execute');", Param{Name: "context", Type: "SchedulableContext"})
					default:
						class.Interfaces = []string{"Database.Batchable<SObject>"}
						context := Param{Name: "context", Type: "Database.BatchableContext"}
						add("start", "Iterable<SObject>", "System.debug('start'); return new List<SObject>{new Account()};", context)
						add("execute", "void", "System.debug('execute');", context, Param{Name: "scope", Type: "List<SObject>"})
						add("finish", "void", "System.debug('finish');", context)
						want = 3
					}
					if err := machine.RegisterClass(class); err != nil {
						t.Fatal(err)
					}
					result := Result{}
					if err := machine.runAsyncJob(AsyncJob{ID: "707000000000001", Kind: kind, Object: Object(class.Name), BatchSize: 200}, &result); err != nil {
						t.Fatal(err)
					}
					if len(result.Debug) != want {
						t.Fatalf("callbacks=%v, want %d", result.Debug, want)
					}
				})
			}
		}
	}
}

// Passive contexts keep their existing empty-id contract; the native matrix
// above and in apextest covers contexts supplied to live callbacks.
func TestAsyncPassiveContextEmptyIDControls(t *testing.T) {
	for _, tc := range []struct {
		route, getter string
		context       Value
	}{
		{"Queueable", "getJobId", asyncContext("QueueableContext", "")},
		{"Batchable", "getJobId", asyncContext("Database.BatchableContext", "")},
		{"Scheduled", "getTriggerId", schedulableContext("")},
		{"Finalizer", "getAsyncApexJobId", finalizerContext("", nil)},
	} {
		t.Run(tc.route, func(t *testing.T) {
			value, _, _, handled, err := New(nil).callPlatformObjectMember(tc.context, tc.getter, nil, &Result{})
			if err != nil || !handled || value.Kind != ValueString || value.Text != "" {
				t.Fatalf("passive getter=%+v; handled=%t error=%v", value, handled, err)
			}
		})
	}
}
