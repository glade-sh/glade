package vm

import (
	"strings"
	"testing"
)

// BenchmarkFieldDescribeAcrossVMs describes the same fields once per fresh VM
// over one org generation (cold) and again in the same VM (warm). The gap is
// the most a per-generation field describe share could save per VM.
func BenchmarkFieldDescribeAcrossVMs(b *testing.B) {
	var sb strings.Builder
	for i := 0; i < 20; i++ {
		sb.WriteString("Schema.DescribeFieldResult d")
		sb.WriteString(string(rune('a' + i)))
		sb.WriteString(" = Widget__c.pkg__Field_000")
		sb.WriteString(string(rune('0' + i%10)))
		sb.WriteString("__c.getDescribe(); String n")
		sb.WriteString(string(rune('a' + i)))
		sb.WriteString(" = d")
		sb.WriteString(string(rune('a' + i)))
		sb.WriteString(".getName();\n")
	}
	program, err := CompileAnonymous(sb.String())
	if err != nil {
		b.Fatal(err)
	}
	org := describeOverlayOrg(200)
	for _, mode := range []string{"cold", "warm"} {
		b.Run(mode, func(b *testing.B) {
			machine := New(nil)
			machine.SetOrg(&org)
			if mode == "warm" {
				if _, err := machine.Execute(program); err != nil {
					b.Fatal(err)
				}
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if mode == "cold" {
					machine = New(nil)
					machine.SetOrg(&org)
				}
				if _, err := machine.Execute(program); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
