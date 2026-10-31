package vm

import (
	"errors"
	"testing"
)

// Supplemental entry-route coverage for the captured synchronous error text.
func TestCPUFollowupMessageEntryRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "named", "istest", "static", "lwc", "aura", "vf", "vf-controller"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				body, err := CompileAnonymousWithOptions(`Integer value = 1;`, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				action, err := CompileAnonymousWithOptions(`Integer value = 1; return new PageReference('/apex/Cpu');`, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				machine := New(nil)
				machine.SetLimitMode(LimitModeStrict)
				org := testDataOrg()
				machine.SetOrg(&org)
				class := Class{Name: "CpuEntry", Methods: map[string]Method{
					"run":    {Name: "CpuEntry.run", ClassName: "CpuEntry", ReturnType: "void", IsStatic: true, Modifiers: []string{"public", "AuraEnabled"}, Program: body},
					"action": {Name: "CpuEntry.action", ClassName: "CpuEntry", ReturnType: "PageReference", Modifiers: []string{"public"}, Program: action},
				}}
				if err := machine.RegisterClass(class); err != nil {
					t.Fatal(err)
				}
				machine.cpuBudgetUsed = (machine.limitCaps.CPUTimeMS+1)*cpuBudgetUnitsPerMillisecond - 1
				var ui UIInvocationResult
				switch route {
				case "anonymous":
					_, err = machine.Execute(body)
				case "named":
					_, err = machine.ExecuteInClass(body, "CpuEntry")
				case "istest":
					_, err = machine.ExecuteTestInClass(body, "CpuEntry")
				case "static":
					_, err = machine.CallStatic("CpuEntry.run", nil)
				case "lwc":
					ui, err = machine.InvokeLWCMethod("CpuEntry", "run", nil)
				case "aura":
					ui, err = machine.InvokeAuraAction("CpuEntry", "run", nil)
				case "vf":
					ui, err = machine.InvokeVisualforceAction("CpuEntry", "action", "/apex/Cpu", nil)
				case "vf-controller":
					_, _, ui, err = machine.InvokeVisualforceActionOnController(Object("CpuEntry"), "CpuEntry", "action", "/apex/Cpu", nil)
				}
				if route == "lwc" || route == "aura" || route == "vf" || route == "vf-controller" {
					if err != nil || ui.Success || ui.Error == nil || ui.Error.Message != "Apex CPU time limit exceeded" {
						t.Fatalf("UI result=%+v err=%v", ui, err)
					}
					return
				}
				var limitErr *RuntimeError
				if !errors.As(err, &limitErr) || limitErr.Type != "System.LimitException" || limitErr.Message != "Apex CPU time limit exceeded" {
					t.Fatalf("route error=%v", err)
				}
			})
		}
	}
}
