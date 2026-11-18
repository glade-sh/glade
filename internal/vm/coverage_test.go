package vm

import (
	"fmt"
	"reflect"
	"sort"
	"testing"

	"github.com/glade-sh/glade/internal/ir"
)

type coverageEvents struct {
	statements, iterations, inits, props []string
}

func (c *coverageEvents) CoverStatement(file string, line, column int) {
	c.statements = append(c.statements, fmt.Sprintf("%s:%d:%d", file, line, column))
}
func (c *coverageEvents) CoverIteration(file string, line, column int) {
	c.iterations = append(c.iterations, fmt.Sprintf("%s:%d:%d", file, line, column))
}
func (c *coverageEvents) CoverClassInit(class string, static bool) {
	c.inits = append(c.inits, fmt.Sprintf("%s:%v", class, static))
}
func (c *coverageEvents) CoverProperty(class, name string) {
	c.props = append(c.props, class+"#"+name)
}

func sortedUnique(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}

// The VM reports a statement's start, a conditionally evaluated operand only
// when it runs, the method entry, a caught exception's catch clause and a
// tested switch label. Positions are the body's own line and column.
func TestCoverageSinkReportsExecutedPositions(t *testing.T) {
	body := "Boolean a = false;\n" +
		"Boolean b = a\n" +
		"    && true;\n" +
		"Integer t = a ? 1\n" +
		"    : 2;\n" +
		"try { throw new DmlException('x'); }\n" +
		"catch (DmlException e) { t = 3; }\n" +
		"switch on t {\n" +
		"    when 1 { t = 4; }\n" +
		"    when 3 { t = 5; }\n" +
		"}\n"
	program, err := CompileAnonymous(body)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "Cov"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(Method{Name: "Cov.run", ClassName: "Cov", ReturnType: "void", IsStatic: true, Program: program, File: "Cov.cls", Line: 40, Column: 5}); err != nil {
		t.Fatal(err)
	}
	call, err := CompileAnonymous("Cov.run();")
	if err != nil {
		t.Fatal(err)
	}
	events := &coverageEvents{}
	machine.SetCoverage(events)
	if _, err := machine.Execute(call); err != nil {
		t.Fatal(err)
	}
	got := sortedUnique(events.statements)
	want := sortedUnique([]string{
		"Cov.cls:40:5",                                             // entry
		"Cov.cls:1:1", "Cov.cls:2:1", "Cov.cls:4:1", "Cov.cls:5:7", // `: 2` ran, `&& true` and `? 1` did not
		"Cov.cls:6:1", "Cov.cls:6:7", "Cov.cls:7:1", "Cov.cls:7:26",
		"Cov.cls:8:1", "Cov.cls:9:5", "Cov.cls:10:5", "Cov.cls:10:14",
	})
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("statements\n got %v\nwant %v", got, want)
	}
	if !contains(events.inits, "Cov:true") {
		t.Fatalf("class init events %v", events.inits)
	}
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// Coverage off is the default and a clone never inherits a sink.
func TestCoverageSinkOffByDefault(t *testing.T) {
	machine := New(nil)
	if machine.coverage != nil {
		t.Fatal("new VM has a coverage sink")
	}
	machine.SetCoverage(&coverageEvents{})
	if clone := machine.CloneRuntime(nil); clone.coverage != nil {
		t.Fatal("clone inherited the coverage sink")
	}
}

// The compiler records the start of each conditionally evaluated operand and
// nothing else; evaluation never reads it.
func TestCompilerRecordsLazyOperandStarts(t *testing.T) {
	program, err := CompileAnonymous("Boolean r = x && y || z;\nInteger n = c ? p : q;\nInteger m = v ?? w;\n")
	if err != nil {
		t.Fatal(err)
	}
	starts := map[string]int32{}
	var walk func(e ir.Expr)
	walk = func(e ir.Expr) {
		if e.StartPos != 0 {
			starts[e.Name] = e.StartPos
		}
		if e.Left != nil {
			walk(*e.Left)
		}
		if e.Right != nil {
			walk(*e.Right)
		}
		for _, a := range e.Args {
			walk(a)
		}
	}
	for _, inst := range program.Instructions {
		walk(inst.Expr)
	}
	want := map[string]int32{"y": 17, "z": 22, "p": 41, "q": 45, "w": 65}
	if !reflect.DeepEqual(starts, want) {
		t.Fatalf("operand starts %v want %v", starts, want)
	}
}
