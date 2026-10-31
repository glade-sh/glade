package vm

import (
	"encoding/csv"
	"errors"
	"os"
	"testing"
)

// The existing native exhaustion captures report the message on both API floors.
func TestCPUFollowupExhaustionMessage(t *testing.T) {
	for _, api := range []string{"62", "67"} {
		t.Run(api, func(t *testing.T) {
			f, err := os.Open("testdata/conformance/cpu_time/native/api" + api + "/org.tsv")
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			r := csv.NewReader(f)
			r.Comma = '\t'
			rows, err := r.ReadAll()
			if err != nil {
				t.Fatal(err)
			}
			expected := rows[6][3]
			machine := New(nil)
			machine.SetLimitMode(LimitModeStrict)
			machine.cpuBudgetUsed = machine.limitCaps.CPUTimeMS * cpuBudgetUnitsPerMillisecond
			err = machine.incrementCPUBudget(cpuBudgetUnitsPerMillisecond)
			var limitErr *RuntimeError
			if !errors.As(err, &limitErr) || limitErr.Message != expected {
				t.Fatalf("CPU exhaustion = %v, want message %q", err, expected)
			}
		})
	}
}

// Bare anonymous futures run inline in the caller's CPU window locally.
func TestCPUFollowupBareFutureSharesBudget(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			machine := New(nil)
			machine.SetLimitMode(LimitModeStrict)
			body, err := CompileAnonymousWithOptions(`System.assertEquals(10000, Limits.getLimitCpuTime()); System.assert(Limits.getCpuTime() >= 20); System.debug('future ran');`, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			err = machine.RegisterClass(Class{Name: "CpuFuture", Methods: map[string]Method{"run": {Name: "CpuFuture.run", ClassName: "CpuFuture", ReturnType: "void", IsStatic: true, Modifiers: []string{"future"}, Program: body}}})
			if err != nil {
				t.Fatal(err)
			}
			program, err := CompileAnonymousWithOptions(`Integer until = Limits.getCpuTime() + 20; while (Limits.getCpuTime() < until) {} CpuFuture.run();`, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			result, err := machine.Execute(program)
			if err != nil || len(result.Debug) != 1 {
				t.Fatalf("inline future: %v, debug=%v", err, result.Debug)
			}
		})
	}
}

func TestCPUFollowupFarFutureScheduleDoesNotDrain(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			machine := New(nil)
			machine.SetLimitMode(LimitModeStrict)
			body, err := CompileAnonymousWithOptions(`System.debug('unexpected schedule');`, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			err = machine.RegisterClass(Class{Name: "CpuSchedule", Interfaces: []string{"Schedulable"}, Methods: map[string]Method{"execute": {Name: "CpuSchedule.execute", ClassName: "CpuSchedule", ReturnType: "void", Params: []Param{{Name: "context", Type: "SchedulableContext"}}, Program: body}}})
			if err != nil {
				t.Fatal(err)
			}
			org := testDataOrg()
			machine.SetOrg(&org)
			program, err := CompileAnonymousWithOptions(`System.schedule('cpu', '0 0 0 1 1 ? 2099', new CpuSchedule());`, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			result, err := machine.Execute(program)
			if err == nil {
				err = machine.DrainAsync(&result)
			}
			if err != nil || len(result.Debug) != 0 {
				t.Fatalf("future schedule drained: %v, debug=%v", err, result.Debug)
			}
		})
	}
}

func TestCPUFollowupMessageAcrossLocalCaps(t *testing.T) {
	for _, cap := range []int{2, 40, 10000} {
		machine := New(nil)
		machine.SetLimitMode(LimitModeStrict)
		caps := defaultLimitCaps()
		caps.CPUTimeMS = cap
		machine.SetLimitCaps(caps)
		err := machine.incrementCPUBudget((cap + 1) * cpuBudgetUnitsPerMillisecond)
		var limitErr *RuntimeError
		if !errors.As(err, &limitErr) || limitErr.Type != "System.LimitException" || limitErr.Message != "Apex CPU time limit exceeded" {
			t.Fatalf("cap %d: %v", cap, err)
		}
	}
	machine := New(nil)
	machine.SetLimitMode(LimitModeStrict)
	err := machine.checkLimit("dmlRows", 3, 2)
	var limitErr *RuntimeError
	if !errors.As(err, &limitErr) || limitErr.Message != "Too many dmlRows: 3 out of 2" {
		t.Fatalf("other governor message changed: %v", err)
	}
}
