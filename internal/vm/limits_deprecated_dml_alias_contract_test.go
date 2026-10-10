package vm

import "testing"

// Primary29336B/SHAfbdb6253efe15b6e94869e9bf08182d6cbe351d1e874285267b460e9fec67723.
// Same-value clauses693/710,727/744,761/778: catalog2204 pairs47/37,
// 48/38,49/39 target DMLStatements getters10/25.
// One Integer binding/target-stability control, then three alias pairs.
// Four fresh programs/eight authored assertions, reach unknown. No DML.
// Expectations are same-checkpoint relative equality, not numeric limits.
// This no-DML profile discriminates current cap aliases; nonzero used-counter
// behavior, reset, enforcement, async, native/API interval/AC7 remain unqualified.
func TestExecLimitsDeprecatedDMLAliasesAPI67(t *testing.T) {
	cases := []struct {
		name string
		source string
	}{
		{name: "integerGetterBindingAndTargetStabilityControl", source: `Integer used=Limits.getDMLStatements();Integer cap=Limits.getLimitDMLStatements();Integer runAs=Limits.getRunAs();Integer runAsCap=Limits.getLimitRunAs();Integer rollbacks=Limits.getSavepointRollbacks();Integer rollbackCap=Limits.getLimitSavepointRollbacks();Integer savepoints=Limits.getSavepoints();Integer savepointCap=Limits.getLimitSavepoints();System.assertEquals(used,Limits.getDMLStatements());System.assertEquals(cap,Limits.getLimitDMLStatements());`},
		{name: "runAsAliasesAtSameCheckpoint", source: `Integer used=Limits.getDMLStatements();Integer cap=Limits.getLimitDMLStatements();System.assertEquals(used,Limits.getRunAs());System.assertEquals(cap,Limits.getLimitRunAs());`},
		{name: "savepointRollbackAliasesAtSameCheckpoint", source: `Integer used=Limits.getDMLStatements();Integer cap=Limits.getLimitDMLStatements();System.assertEquals(used,Limits.getSavepointRollbacks());System.assertEquals(cap,Limits.getLimitSavepointRollbacks());`},
		{name: "savepointAliasesAtSameCheckpoint", source: `Integer used=Limits.getDMLStatements();Integer cap=Limits.getLimitDMLStatements();System.assertEquals(used,Limits.getSavepoints());System.assertEquals(cap,Limits.getLimitSavepoints());`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(tc.source, CompileOptions{APIVersion: "67.0"})
			if err != nil {
				t.Fatal(err)
			}
			if program.APIVersion != "67.0" {
				t.Fatalf("compiled API version = %q, want 67.0", program.APIVersion)
			}
			caps, ok := LimitCapsForProfile("strict-sync")
			if !ok {
				t.Fatal("existing strict-sync local cap profile is unavailable")
			}
			machine := New(nil)
			machine.SetLimitMode(LimitModeStrict)
			machine.SetLimitCaps(caps)
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
