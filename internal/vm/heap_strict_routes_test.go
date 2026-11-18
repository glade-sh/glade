package vm_test

import (
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// strictHeapRouteBodies are slices of the shape generator and of the review
// probes that run in a method frame, a trigger body and a Queueable. They hold
// no DML, so a trigger body cannot recurse; Ids are literals.
func strictHeapRouteBodies() map[string]string {
	noInsert := func(s string) string {
		s = strings.Replace(s, "new Account(Name = 'acct' + i, Description = 'd' + i)", "new Account(Id = '0010000000' + String.valueOf(10000 + i), Name = 'acct' + i, Description = 'd' + i)", 1)
		return strings.Replace(s, "insert accs;\n", "", 1)
	}
	bodies := map[string]string{}
	for _, seed := range []int64{0, 1, 4, 7} {
		bodies[fmt.Sprintf("gen%d", seed)] = noInsert(strictShapeProgram(rand.New(rand.NewSource(seed)), 40))
	}
	for _, name := range []string{
		"loopvar-reassigned-then-rebound",
		"loopvar-set-iterable",
		"fresh-id-shared-then-rebind",
		"fresh-id-shared-then-field-overwrite",
		"fresh-id-into-new-record-then-drop",
	} {
		bodies["probe-"+name] = strictHeapReviewProbes[name]
	}
	return bodies
}

func strictHeapRouteProject(t *testing.T, body string) (*vm.VM, typesys.Index, vm.CompileOptions) {
	t.Helper()
	root := t.TempDir()
	write := func(name, text string) {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("sfdx-project.json", `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	const classes = "force-app/main/default/classes/"
	write(classes+"HeapRouteBody.cls", "public class HeapRouteBody {\n    public static void run() {\n"+body+"\n    }\n}")
	write(classes+"HeapRouteJob.cls", `public class HeapRouteJob implements Queueable {
    public void execute(QueueableContext context) { HeapRouteBody.run(); }
}`)
	write(classes+"HeapRouteTest.cls", `@IsTest private class HeapRouteTest {
    @IsTest static void direct() { HeapRouteBody.run(); }
    @IsTest static void queued() { Test.startTest(); System.enqueueJob(new HeapRouteJob()); Test.stopTest(); }
}`)
	for _, name := range []string{"HeapRouteBody", "HeapRouteJob", "HeapRouteTest"} {
		write(classes+name+".cls-meta.xml", "<ApexClass><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>")
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	s, err := schema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	index := typesys.Build(p, s)
	if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
		t.Fatalf("project diagnostics: %v %v\n%s", index.Diagnostics, analysis.Diagnostics, body)
	}
	org := apextest.OrgFromIndex(index)
	org.APIVersion = "67.0"
	machine := vm.New(nil)
	machine.SetOrg(&org)
	if err := apextest.RegisterProjectRuntime(machine, index); err != nil {
		t.Fatal(err)
	}
	return machine, index, vm.CompileOptions{APIVersion: "67.0"}
}

func runStrictHeapRoute(t *testing.T, route, body string, caps vm.LimitCaps, plansOff bool) (strictShapeOutcome, int) {
	t.Helper()
	defer vm.SetHeapStatementPlansOffForTest(plansOff)()
	machine, index, opts := strictHeapRouteProject(t, body)
	source := "HeapRouteBody.run();"
	switch route {
	case "method":
		machine.EnableTestContext()
		if err := apextest.RegisterTestMethodForRequest(machine, index, "HeapRouteTest", "direct"); err != nil {
			t.Fatal(err)
		}
		source = "HeapRouteTest.direct();"
	case "queueable":
		machine.EnableTestContext()
		if err := apextest.RegisterTestMethodForRequest(machine, index, "HeapRouteTest", "queued"); err != nil {
			t.Fatal(err)
		}
		source = "HeapRouteTest.queued();"
	case "trigger":
		program, err := vm.CompileAnonymousWithOptions(body, opts)
		if err != nil {
			t.Fatalf("trigger body: %v\n%s", err, body)
		}
		for _, timing := range []string{"before", "after"} {
			if err := machine.RegisterTrigger(vm.Trigger{Name: "HeapRouteTrigger", Object: "Account", Timing: timing, Operation: "insert", APIVersion: "67.0", Program: program}); err != nil {
				t.Fatal(err)
			}
		}
		source = "insert new Account(Name = 'seed');"
	}
	program, err := vm.CompileAnonymousWithOptions(source, opts)
	if err != nil {
		t.Fatal(err)
	}
	machine.SetLimitMode(vm.LimitModeStrict)
	machine.SetLimitCaps(caps)
	result, runErr := machine.Execute(program)
	out := strictShapeOutcome{debug: result.Debug, limits: result.Limits, violations: result.LimitViolations}
	if runErr != nil {
		out.err = runErr.Error()
		var rt *vm.RuntimeError
		if errors.As(runErr, &rt) {
			out.errType = rt.Type
			// Each run has its own project directory.
			for _, frame := range rt.Stack {
				frame.File = filepath.Base(frame.File)
				out.stack = append(out.stack, frame)
			}
		}
	}
	return out, machine.HeapChecksPricedForTest()
}

// The priced outcome in a class method frame called from a test method, in a
// before and after insert trigger body and in a Queueable drained at
// Test.stopTest equals the full walk, under no cap and two caps that stop the
// run part way. Verify mode compares every priced check with the walk.
func TestStrictHeapRoutesMatchFullWalk(t *testing.T) {
	t.Cleanup(apextest.InvalidateRuntimeCaches)
	sync, _ := vm.LimitCapsForProfile("default")
	for name, body := range strictHeapRouteBodies() {
		for _, route := range []string{"method", "trigger", "queueable"} {
			t.Run(route+"/"+name, func(t *testing.T) {
				base, _ := runStrictHeapRoute(t, route, body, sync, true)
				if base.err != "" && !strings.Contains(base.err, "Too many heapSize") {
					t.Logf("uncapped err = %q", base.err)
				}
				ran, priced := runStrictHeapRoute(t, route, body, sync, false)
				if !strings.Contains(strings.Join(ran.debug, "\n"), "end ") || ran.err != "" {
					t.Fatalf("body did not run to its end: err=%q debug=%v", ran.err, ran.debug)
				}
				if priced < 3 {
					t.Fatalf("only %d checks priced in the %s route", priced, route)
				}
				// The method frame and the trigger report no final heap in the
				// result, so scale the caps by the largest number the body
				// printed, which is its last Limits.getHeapSize sample.
				end := 0
				for _, line := range ran.debug {
					for _, field := range strings.Fields(line) {
						var n int
						if _, err := fmt.Sscanf(field, "%d", &n); err == nil && n > end {
							end = n
						}
					}
				}
				stopped := 0
				for _, cap := range []int{0, end * 2 / 3, end - 1} {
					c := sync
					if cap != 0 {
						c.HeapSize = cap
					}
					want, _ := runStrictHeapRoute(t, route, body, c, true)
					got, _ := runStrictHeapRoute(t, route, body, c, false)
					if !reflect.DeepEqual(got, want) {
						t.Fatalf("cap %d: priced outcome differs from full walk:\n got  %+v\n want %+v", cap, got, want)
					}
					if strings.Contains(want.err, "Too many heapSize: ") {
						stopped++
					}
				}
				if stopped == 0 {
					t.Fatalf("no capped run threw the heap LimitException (end heap %d)", end)
				}
			})
		}
	}
}
