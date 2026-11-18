package vm

import "testing"

// BenchmarkGovernorCounters measures the per-operation counter path that every
// SOQL query, DML statement and row charge crosses.
func BenchmarkGovernorCounters(b *testing.B) {
	for _, name := range []string{"queries", "queryRows", "dmlStatements", "dmlRows", "aggregateQueries", "callouts", "runAs", "savepoints", "queueableJobs"} {
		b.Run(name, func(b *testing.B) {
			machine := New(nil)
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if i%40 == 0 {
					machine.limits = Limits{}
					machine.limitViolations = nil
				}
				if err := machine.incrementLimit(name, 1); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
