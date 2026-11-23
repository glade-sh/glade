package vm

import (
	"fmt"
	"strings"
	"testing"
)

// BenchmarkFieldDescribeAcrossVMs describes the same fields once per fresh VM
// over one org generation (cold) and again in the same VM (warm). The gap is
// the most a per-generation field describe share could save per VM.
func BenchmarkFieldDescribeAcrossVMs(b *testing.B) {
	var sb strings.Builder
	for i := 0; i < 20; i++ {
		fmt.Fprintf(&sb, "Schema.DescribeFieldResult x%d = Widget__c.pkg__Field_%04d__c.getDescribe(); String y%d = x%d.getName();\n", i, i, i, i)
	}
	program, err := CompileAnonymous(sb.String())
	if err != nil {
		b.Fatal(err)
	}
	empty, err := CompileAnonymous("Integer z = 1;")
	if err != nil {
		b.Fatal(err)
	}
	org := describeOverlayOrg(200)
	b.Run("coldbase", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			machine := New(nil)
			machine.SetOrg(&org)
			if _, err := machine.Execute(empty); err != nil {
				b.Fatal(err)
			}
		}
	})
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
