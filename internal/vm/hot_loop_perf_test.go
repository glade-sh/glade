package vm

import (
	"fmt"
	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/trace"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestStringSliceCachedMemberRoutes(t *testing.T) {
	for _, method := range []string{"mid", "left", "right"} {
		t.Run(method, func(t *testing.T) {
			machine := New(nil)
			text := String(strings.Repeat("abcd", 512))
			args := []Value{Int(1)}
			if method == "mid" {
				args = append(args, Int(1))
			}
			for i := 0; i < 20; i++ {
				if _, handled, err := machine.callValueMember("", text, method, args, nil); err != nil || !handled {
					t.Fatalf("handled=%v err=%v", handled, err)
				}
			}
			if len(machine.stringASCII) != 1 {
				t.Fatalf("repeated %s bypassed bounded ASCII classification cache: entries=%d", method, len(machine.stringASCII))
			}
		})
	}
}

func TestStringSliceCachedAndUncachedOutcomeRows(t *testing.T) {
	for textIndex, text := range []string{"", "abc", strings.Repeat("abcd", 128), strings.Repeat("abcd", 128) + "Ω😀"} {
		for _, method := range []string{"mid", "MiD", "left", "LEFT", "right", "Right"} {
			for argsIndex, args := range [][]Value{nil, {Int(-1)}, {Int(0)}, {Int(1)}, {Int(512)}, {Int(999)}, {Int(-1), Int(2)}, {Int(0), Int(-1)}, {Int(1), Int(2)}, {Int(512), Int(999)}, {Int(999), Int(1)}, {Null}, {String("bad")}, {Int(1), Null}, {Int(1), Int(2), Int(3)}} {
				t.Run(fmt.Sprintf("text%d/%s/args%d", textIndex, method, argsIndex), func(t *testing.T) {
					want, wh, we := callStringMember(String(text), method, args)
					machine := New(nil)
					got, gh, ge := machine.callValueMember("", String(text), method, args, nil)
					if got.Kind != want.Kind || got.Text != want.Text || gh != wh || errorText(ge) != errorText(we) {
						t.Fatalf("got %v/%v/%v want %v/%v/%v", got, gh, ge, want, wh, we)
					}
					t.Logf("ROW string/%d/%s/%d %s %q %v %q", textIndex, method, argsIndex, got.Kind, got.Text, gh, errorText(ge))
				})
			}
		}
	}
}

func TestHotLoopProfileO1StringSlices(t *testing.T) {
	machine := New(nil)
	text := String(strings.Repeat("abcd", 50000))
	sum := 0
	for i := 0; i < 20000; i++ {
		for _, method := range []string{"mid", "left", "right"} {
			args := []Value{Int(1)}
			if method == "mid" {
				args = []Value{Int(int64(i)), Int(1)}
			}
			got, handled, err := machine.callValueMember("", text, method, args, nil)
			if err != nil || !handled {
				t.Fatal(err)
			}
			sum += len(got.Text)
		}
	}
	if sum != 60000 {
		t.Fatal(sum)
	}
	t.Logf("ROW O1 length=%d", sum)
}

func hotLoopXML(count int) string {
	return "<root>" + strings.Repeat(`<item key="v">abcdefghijklmnopqrst</item>`, count) + "</root>"
}

func TestHotLoopProfileO3DomLoad(t *testing.T) {
	doc, err := parseDomDocument(hotLoopXML(10000))
	if err != nil {
		t.Fatal(err)
	}
	children := domChildElements(doc.Fields["root"])
	if len(children.List) != 10000 {
		t.Fatal(len(children.List))
	}
	for _, child := range children.List {
		if domNodeList(child, "children").List[0].Fields["text"].Text != "abcdefghijklmnopqrst" {
			t.Fatal("lost text")
		}
	}
	t.Logf("ROW O3 children=%d first=%s last=%s", len(children.List), children.List[0].Fields["name"].Text, children.List[9999].Fields["name"].Text)
}

func TestHotLoopProfileO2ListArithmetic(t *testing.T)         { hotLoopListArithmetic(t, true) }
func TestHotLoopProfileO2ListArithmeticUntraced(t *testing.T) { hotLoopListArithmetic(t, false) }
func hotLoopListArithmetic(t *testing.T, tracing bool) {
	program, err := CompileAnonymous(`List<Integer> pixels=new List<Integer>{1,2,3,4}; Integer checksum=0; for(Integer i=0;i<40000;i++){Integer at=i&3; Integer value=(pixels.get(at)+i)&255; pixels.set(at,value); checksum=checksum+value;} System.assertEquals(4838224,checksum); System.assertEquals(225,pixels.get(0)); System.assertEquals(242,pixels.get(1)); System.assertEquals(3,pixels.get(2)); System.assertEquals(20,pixels.get(3)); System.debug('ROW O2 '+checksum+':'+pixels);`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.SetTraceEnabled(tracing)
	result, err := machine.Execute(program)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range result.Debug {
		t.Log(line)
	}
}

// TestHotLoopProfileNull is the measurement control: process start-up,
// compilation and one trivial statement, with no hot loop.
func TestHotLoopProfileNull(t *testing.T) {
	program, err := CompileAnonymous(`Integer i=0;`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
	t.Log("ROW Null")
}

// TestHotLoopProfileO2ListArithmeticLarge sizes the untraced list/arithmetic
// loop so the interpreter dominates the process. The expected checksum and
// pixels come from the same arithmetic in Go, not from a pinned constant.
func TestHotLoopProfileO2ListArithmeticLarge(t *testing.T) {
	const iterations = 400000
	pixels := []int64{1, 2, 3, 4}
	var checksum int64
	for i := int64(0); i < iterations; i++ {
		at := i & 3
		value := (pixels[at] + i) & 255
		pixels[at] = value
		checksum += value
	}
	program, err := CompileAnonymous(fmt.Sprintf(`List<Integer> pixels=new List<Integer>{1,2,3,4}; Integer checksum=0; for(Integer i=0;i<%d;i++){Integer at=i&3; Integer value=(pixels.get(at)+i)&255; pixels.set(at,value); checksum=checksum+value;} System.assertEquals(%d,checksum); System.assertEquals(%d,pixels.get(0)); System.assertEquals(%d,pixels.get(1)); System.assertEquals(%d,pixels.get(2)); System.assertEquals(%d,pixels.get(3)); System.debug('ROW O2L '+checksum+':'+pixels);`, iterations, checksum, pixels[0], pixels[1], pixels[2], pixels[3]))
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.SetTraceEnabled(false)
	result, err := machine.Execute(program)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range result.Debug {
		t.Log(line)
	}
}

func TestXMLTokenDOMAndReaderOutcomeRows(t *testing.T) {
	baseline, err := os.ReadFile("testdata/xml_token_outcomes.txt")
	if err != nil {
		t.Fatal(err)
	}
	expected := strings.Split(strings.TrimSpace(string(baseline)), "\n")
	sources := []string{
		`<root/>`, `<?xml version="1.0"?><root>
Ω😀<x a="v"/>tail</root>`,
		`<r xmlns:n="urn:n"><n:x n:a="v">a&amp;b<!--c--><![CDATA[d]]></n:x></r>`,
		`<r><x></r>`, `<r a="1" a="2"/>`, `<r><n:x/></r>`, `<r>`, ``, `<r/>oops`,
	}
	for _, dom := range []bool{false, true} {
		for i, source := range sources {
			t.Run(fmt.Sprintf("dom=%v/source%d", dom, i), func(t *testing.T) {
				tokens, err := xmlReadTokens(source, dom)
				rows := []string{fmt.Sprintf("ROW xml/%v/%d error=%q", dom, i, errorText(err))}
				for j, token := range tokens {
					rows = append(rows, fmt.Sprintf("ROW xml/%v/%d/%d kind=%s name=%s ns=%s text=%q location=%q startLine=%d", dom, i, j, xmlStreamReaderTokenKind(token), token.Fields["localName"].Text, token.Fields["namespace"].Text, token.Fields["text"].Text, token.Fields["location"].Text, token.Fields["startLine"].Int))
				}
				if dom && err == nil {
					doc, err := parseDomDocument(source)
					if err != nil {
						t.Fatal(err)
					}
					rows = append(rows, fmt.Sprintf("ROW dom/%d xml=%q", i, domDocumentXMLString(doc)))
				}
				var want []string
				for _, row := range expected {
					prefix := fmt.Sprintf("ROW xml/%v/%d", dom, i)
					if strings.HasPrefix(row, prefix+" ") || strings.HasPrefix(row, prefix+"/") || dom && strings.HasPrefix(row, fmt.Sprintf("ROW dom/%d ", i)) {
						want = append(want, row)
					}
				}
				if strings.Join(rows, "\n") != strings.Join(want, "\n") {
					t.Fatalf("baseline row mismatch\ngot:\n%s\nwant:\n%s", strings.Join(rows, "\n"), strings.Join(want, "\n"))
				}
				for _, row := range rows {
					t.Log(row)
				}
			})
		}
	}
}

func TestXMLTokenLocationLongPrefixLinearShapeRoutes(t *testing.T) {
	for _, dom := range []bool{false, true} {
		t.Run(fmt.Sprintf("dom=%v", dom), func(t *testing.T) {
			// 1024 tokens make the per-token rescan term dominate: on loaded
			// capture-host the rescanning reader measured 85-232x at 256 tokens while
			// the incremental reader measured 6-18x, too close to the limit.
			small := hotLoopXML(1024)
			large := strings.Repeat(" ", 1<<20) + small
			measure := func(source string) time.Duration {
				best := time.Hour
				for i := 0; i < 3; i++ {
					start := time.Now()
					tokens, err := xmlReadTokens(source, dom)
					elapsed := time.Since(start)
					if err != nil || len(tokens) < 1024 {
						t.Fatalf("tokens=%d err=%v", len(tokens), err)
					}
					if elapsed < best {
						best = elapsed
					}
				}
				return best
			}
			shortTime, longTime := measure(small), measure(large)
			t.Logf("short=%s long-prefix=%s ratio=%.2f", shortTime, longTime, float64(longTime)/float64(shortTime))
			// A fixed token count must not rescan the leading megabyte per token.
			// The generous ratio leaves room for decoding that megabyte once.
			if longTime > shortTime*12 {
				t.Fatalf("token locations rescan long prefix: short=%s long=%s (limit 12x)", shortTime, longTime)
			}
		})
	}
}

func TestSaturatedStatementTraceAllocationAndOutcomeRoute(t *testing.T) {
	machine := New(nil)
	program := ir.Program{Source: "1;", Instructions: []ir.Instruction{{Op: ir.OpExpr, Expr: ir.Expr{Kind: ir.ExprLiteral, Value: "1"}}}}
	result := &Result{traceEnabled: true, Trace: make([]trace.Event, maxTraceEvents+1)}
	result.Trace[0] = trace.Instant("retained", "apex.statement", 0, map[string]any{"marker": "first"})
	result.Trace[maxTraceEvents] = trace.Instant("apex.trace.truncated", "apex.trace", int64(maxTraceEvents), map[string]any{"dropped": 1000, "limit": maxTraceEvents})
	retained := result.Trace[0]
	allocations := testing.AllocsPerRun(20, func() {
		result.Trace[maxTraceEvents].Args["dropped"] = 1000
		if _, err := machine.executeProgram(program, result); err != nil {
			t.Fatal(err)
		}
		if len(result.Trace) != maxTraceEvents+1 || result.Trace[maxTraceEvents].Args["dropped"] != 1001 {
			t.Fatal("changed saturated trace outcome")
		}
	})
	if !reflect.DeepEqual(result.Trace[0], retained) {
		t.Fatal("changed retained trace event")
	}
	t.Logf("saturated statement trace allocations=%.0f", allocations)
	if allocations > 2 {
		t.Fatalf("discarded statement trace still builds payload: %.0f allocations, want <=2", allocations)
	}
}

func TestBlockScopeSavedBindingOutcomeRoutes(t *testing.T) {
	for _, op := range []ir.Op{ir.OpBlock, ir.OpIf, ir.OpFor, ir.OpForEach, ir.OpTry, ir.OpSwitch} {
		for _, size := range []int{0, 2, 12} {
			t.Run(fmt.Sprintf("%s/locals=%d", op, size), func(t *testing.T) {
				machine := New(nil)
				machine.SetTraceEnabled(false)
				machine.finalLocals = make(map[string]bool)
				block := ir.Instruction{Op: op}
				for i := 0; i < size; i++ {
					name := fmt.Sprintf("local%d", i)
					machine.Globals[name] = longIntValue(99)
					machine.VarTypes[name] = "Long"
					machine.finalLocals[name] = true
					block.Then = append(block.Then, ir.Instruction{Op: ir.OpDeclGroup, Then: []ir.Instruction{{Op: ir.OpDeclare, Name: name, Type: "Integer"}}})
				}
				if _, err := machine.executeBlockScope(ir.Program{}, block, nil); err != nil {
					t.Fatal(err)
				}
				for i := 0; i < size; i++ {
					name := fmt.Sprintf("local%d", i)
					if machine.Globals[name].Int != 99 || machine.VarTypes[name] != "Long" || !machine.finalLocals[name] {
						t.Fatalf("lost saved binding %s", name)
					}
				}
			})
		}
	}
}

func TestStringSliceCacheEvictionRoutes(t *testing.T) {
	machine := New(nil)
	for i := 0; i < 2*stringASCIIMaxEntries; i++ {
		text := String(strings.Repeat(string(rune('a'+i)), 512))
		for _, method := range []string{"mid", "left", "right"} {
			args := []Value{Int(1)}
			if method == "mid" {
				args = append(args, Int(1))
			}
			got, handled, err := machine.callValueMember("", text, method, args, nil)
			want, _, _ := callStringMember(text, method, args)
			if err != nil || !handled || got.Text != want.Text {
				t.Fatalf("%s after eviction: %v/%v/%v", method, got, handled, err)
			}
		}
		if len(machine.stringASCII) > stringASCIIMaxEntries || machine.stringASCIIBytes > stringASCIIMaxBytes {
			t.Fatal("slice route exceeded cache bounds")
		}
	}
}

func TestXMLTokenMultilineUnicodeStartLineRoutes(t *testing.T) {
	for _, dom := range []bool{false, true} {
		tokens, err := xmlReadTokens("<!--😀-->\r\n\n<root>\nΩ\n<x/>\n</root>", dom)
		if err != nil {
			t.Fatal(err)
		}
		starts := 0
		for _, token := range tokens {
			if xmlStreamReaderTokenKind(token) != "START_ELEMENT" {
				continue
			}
			starts++
			want := int64(3)
			if token.Fields["localName"].Text == "x" {
				want = 5
			}
			if token.Fields["startLine"].Int != want {
				t.Fatalf("dom=%v name=%s startLine=%v want=%d", dom, token.Fields["localName"].Text, token.Fields["startLine"], want)
			}
		}
		if starts != 2 {
			t.Fatal(starts)
		}
	}
}

func TestStatementTraceCapBoundaryEnabledAndDisabledRoutes(t *testing.T) {
	program := ir.Program{Source: "1;", Instructions: []ir.Instruction{{Op: ir.OpExpr, Expr: ir.Expr{Kind: ir.ExprLiteral, Value: "1"}}}}
	for _, enabled := range []bool{false, true} {
		for _, size := range []int{0, maxTraceEvents - 1, maxTraceEvents, maxTraceEvents + 1} {
			t.Run(fmt.Sprintf("enabled=%v/size=%d", enabled, size), func(t *testing.T) {
				makeResult := func() *Result {
					result := &Result{traceEnabled: enabled, Trace: make([]trace.Event, size)}
					if size > maxTraceEvents {
						result.Trace[size-1] = trace.Instant("apex.trace.truncated", "apex.trace", int64(maxTraceEvents), map[string]any{"dropped": 14, "limit": maxTraceEvents})
					}
					return result
				}
				got, want := makeResult(), makeResult()
				if enabled {
					appendResultTrace(want, statementTraceEvent(0, program.Instructions[0], program.Source))
				}
				machine := New(nil)
				machine.SetTraceEnabled(enabled)
				if _, err := machine.executeProgram(program, got); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(got.Trace, want.Trace) {
					t.Fatal("trace cap changed retained events or dropped count")
				}
			})
		}
	}
}
