# Regex Real Parity Fix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move the remaining Pattern, Matcher, and String.split regex gaps from partial support to oracle-backed local support for Java character-class algebra, Unicode grapheme clusters, and Apex UTF-16 matcher positions.

**Architecture:** Keep regexp2 as the default engine for ordinary regexes. Add a Java character-class lowering pass for `[A&&B]` algebra, and add an input-aware grapheme pass for `\X` and `\b{g}` that validates matches against Unicode grapheme boundaries. Add a small UTF-16 index foundation because Salesforce matcher spans and String.length count UTF-16 code units.

**Tech Stack:** Go, `github.com/dlclark/regexp2`, `github.com/rivo/uniseg@v0.4.7`, Salesforce CLI against `oaer-probe-max`, and the sibling `glade-tools` compat plugin.

---

## Evidence To Preserve

Use these facts as the acceptance target.

- OpenJDK source may be used as an architectural reference, but do not translate or copy it into Glade. Glade is Apache-2.0, while OpenJDK `Pattern.java` and `Grapheme.java` carry GPLv2 with the Classpath exception. Treat OpenJDK as a map for behavior and decomposition, not as source material.
- The useful OpenJDK shape is:
  - Character classes compile to character predicates.
  - Union, intersection, and negation compose those predicates.
  - `\X` advances to the next Unicode grapheme boundary.
  - `\b{g}` checks whether the current UTF-16 index is on a grapheme boundary.
- Java `Pattern` documents character-class union/intersection precedence, `\X`, and `\b{g}`.
- UAX #29 defines extended grapheme clusters.
- Salesforce scratch org `oaer-probe-max` matched `\X` for CRLF, Hangul Jamo, regional-indicator flags, skin-tone emoji, ZWJ family emoji, combining marks, and isolated marks.
- Salesforce accepted nested class algebra:
  - `[a-z&&[m-p]&&[n-o]]+`
  - `[a-z&&[m-p]&&[^o]]+`
  - `[a-z&&[m-p&&[^o]]]+`
- Salesforce matcher spans count UTF-16 code units:
  - thumbs-up with skin tone: `group().length():start:end` is `4:0:4`
  - family ZWJ sequence: `11:0:11`

## File Structure

Product worktree, usually `/Users/matt/Dev/glade/.worktrees/stdlib-supported`:

- Create `internal/vm/stdlib_apex_string_index.go` for UTF-16 code-unit length and byte-index mapping.
- Create `internal/vm/stdlib_apex_string_index_test.go` for fast unit tests around ASCII, BMP, emoji, and ZWJ strings.
- Create `internal/vm/stdlib_regex_class.go` for Java character-class parsing and regexp2 lowering.
- Create `internal/vm/stdlib_regex_class_test.go` for fast parser/lowering tests.
- Create `internal/vm/stdlib_regex_grapheme.go` for `uniseg` boundary tables, `\X` and `\b{g}` rewriting, internal capture validation, and public group mapping.
- Create `internal/vm/stdlib_regex_grapheme_test.go` for fast boundary and match-validation tests.
- Modify `internal/vm/stdlib.go` for `Pattern.compile`, `Pattern.matches`, `String.fromCharArray`, `String.split`, and shared index helpers.
- Modify `internal/vm/stdlib_string.go` for `String.length`.
- Modify `internal/vm/stdlib_regex.go` for `Pattern.matcher`, `Pattern.split`, `Matcher.find`, `Matcher.matches`, `Matcher.lookingAt`, `Matcher.groupCount`, `Matcher.start`, `Matcher.end`, `Matcher.region`, `Matcher.regionStart`, `Matcher.regionEnd`, replacement, reset, and usePattern.
- Modify `internal/vm/stdlib_regex_engine.go` to route compile/lower/match through the new class and grapheme helpers.
- Modify `internal/vm/regex_test.go` for Apex-level behavior tests.
- Modify `go.mod` and `go.sum` to add `github.com/rivo/uniseg@v0.4.7`.
- Modify `docs/STDLIB_COVERAGE.md` after fixtures regenerate.

Tools worktree, usually `/Users/matt/Dev/glade/.worktrees/stdlib-supported-tools`:

- Modify `internal/oracleprobe/stdlib_cases.go` to capture the new Salesforce oracle rows.
- Modify `internal/oracleprobe/oracleprobe_test.go` to require the new row coverage.
- Add or update `docs/fixtures/oracle/stdlib/core-regex-oracle.json`.
- Add or update fixture files under `docs/fixtures/` for class algebra, grapheme matching, grapheme splitting, UTF-16 spans, and `String.fromCharArray`.
- Modify `internal/capability/stdlib.go` so generated notes no longer claim the old fences once product tests pass.
- Regenerate `docs/generated/stubs/STUB_CONTRACTS.json` if the fixture set changes.

## Parallel Work Split

- Squad A owns oracle probes and glade-tools fixtures.
- Squad B owns UTF-16 index helpers and `String.fromCharArray`/`String.length`.
- Squad C owns Java class algebra lowering.
- Squad D owns grapheme boundary matching.
- Integration owner wires Matcher/String.split APIs, regenerates support docs, and runs broad verification.

Do not have two squads edit `internal/vm/stdlib_regex.go` at the same time. That file is the final integration point.

---

### Task 1: Add Oracle Coverage For The Real Regex Edges

**Files:**
- Modify: `/Users/matt/Dev/glade/.worktrees/stdlib-supported-tools/internal/oracleprobe/stdlib_cases.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/stdlib-supported-tools/internal/oracleprobe/oracleprobe_test.go`
- Create or update: `/Users/matt/Dev/glade/.worktrees/stdlib-supported-tools/docs/fixtures/oracle/stdlib/core-regex-oracle.json`

- [ ] **Step 1: Add oracle cases for grapheme matching, boundaries, spans, and nested class algebra**

Add these cases near the existing Pattern cases in `StdlibCases()`:

```go
{ID: "pattern-grapheme-crlf", Area: "Pattern", API: "Pattern.matches", Mode: ModeAnonymous, Statements: []string{
	"String gladeGX = String.fromCharArray(new List<Integer>{92}) + 'X'",
	"String gladeCRLF = String.fromCharArray(new List<Integer>{13,10})",
}, Expression: "Pattern.matches(gladeGX, gladeCRLF)", ValueType: "Boolean"},
{ID: "pattern-grapheme-zwj-family", Area: "Pattern", API: "Pattern.matches", Mode: ModeAnonymous, Statements: []string{
	"String gladeGX = String.fromCharArray(new List<Integer>{92}) + 'X'",
	"String gladeFamily = String.fromCharArray(new List<Integer>{55357,56424,8205,55357,56425,8205,55357,56423,8205,55357,56422})",
}, Expression: "Pattern.matches(gladeGX, gladeFamily)", ValueType: "Boolean"},
{ID: "matcher-grapheme-span-emoji", Area: "Pattern", API: "Matcher.find", Mode: ModeAnonymous, Statements: []string{
	"String gladeGX = String.fromCharArray(new List<Integer>{92}) + 'X'",
	"String gladeThumb = String.fromCharArray(new List<Integer>{55357,56397,55356,57341})",
	"Matcher gladeM = Pattern.compile(gladeGX).matcher(gladeThumb + 'x')",
	"List<String> gladeSpans = new List<String>()",
	"while (gladeM.find()) { gladeSpans.add(String.valueOf(gladeM.group().length()) + ':' + String.valueOf(gladeM.start()) + ':' + String.valueOf(gladeM.end())); };",
}, Expression: "JSON.serialize(gladeSpans)", ValueType: "String"},
{ID: "matcher-grapheme-boundary", Area: "Pattern", API: "Matcher.find", Mode: ModeAnonymous, Statements: []string{
	"String gladeBG = String.fromCharArray(new List<Integer>{92}) + 'b{g}'",
	"String gladeMark = String.fromCharArray(new List<Integer>{769})",
	"Matcher gladeM = Pattern.compile(gladeBG).matcher('e' + gladeMark + 'x')",
	"List<String> gladeSpans = new List<String>()",
	"while (gladeM.find()) { gladeSpans.add(String.valueOf(gladeM.start()) + ':' + String.valueOf(gladeM.end())); };",
}, Expression: "JSON.serialize(gladeSpans)", ValueType: "String"},
{ID: "pattern-class-intersection-chain", Area: "Pattern", API: "Pattern.matches", Mode: ModeAnonymous, Expression: "Pattern.matches('[a-z&&[m-p]&&[n-o]]+', 'no')", ValueType: "Boolean"},
{ID: "pattern-class-intersection-chain-reject", Area: "Pattern", API: "Pattern.matches", Mode: ModeAnonymous, Expression: "Pattern.matches('[a-z&&[m-p]&&[n-o]]+', 'mp')", ValueType: "Boolean"},
{ID: "pattern-class-intersection-positive-negative", Area: "Pattern", API: "Pattern.matches", Mode: ModeAnonymous, Expression: "Pattern.matches('[a-z&&[m-p]&&[^o]]+', 'mnp')", ValueType: "Boolean"},
{ID: "pattern-class-intersection-nested", Area: "Pattern", API: "Pattern.matches", Mode: ModeAnonymous, Expression: "Pattern.matches('[a-z&&[m-p&&[^o]]]+', 'mnp')", ValueType: "Boolean"},
{ID: "string-from-char-array-utf16-pair", Area: "String", API: "String.fromCharArray", Mode: ModeAnonymous, Expression: "String.fromCharArray(new List<Integer>{55357,56832}).length()", ValueType: "Integer"},
{ID: "string-from-char-array-scalar-truncates", Area: "String", API: "String.fromCharArray", Mode: ModeAnonymous, Expression: "String.fromCharArray(new List<Integer>{128512}).length()", ValueType: "Integer"},
```

- [ ] **Step 2: Tighten the oracle coverage test**

Add required IDs to `TestStdlibCasesCoverTargetRows` with a second set for edge rows:

```go
wantIDs := map[string]bool{
	"pattern-grapheme-crlf":                         true,
	"pattern-grapheme-zwj-family":                   true,
	"matcher-grapheme-span-emoji":                   true,
	"matcher-grapheme-boundary":                     true,
	"pattern-class-intersection-chain":              true,
	"pattern-class-intersection-chain-reject":       true,
	"pattern-class-intersection-positive-negative":  true,
	"pattern-class-intersection-nested":             true,
	"string-from-char-array-utf16-pair":             true,
	"string-from-char-array-scalar-truncates":       true,
}
for _, tc := range cases {
	delete(wantIDs, tc.ID)
}
if len(wantIDs) != 0 {
	t.Fatalf("missing regex oracle edge cases: %#v", wantIDs)
}
```

- [ ] **Step 3: Run the focused tools tests**

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported-tools
go test ./internal/oracleprobe -count=1
```

Expected: `ok`.

- [ ] **Step 4: Refresh the oracle fixture from Salesforce**

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported-tools
go run ./cmd/glade-tools oracle-stdlib --target-org oaer-probe-max --output docs/fixtures/oracle/stdlib/core-regex-oracle.json
```

Expected: command exits `0`, and the fixture contains the new IDs with values matching the evidence section.

- [ ] **Step 5: Commit tools oracle evidence**

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported-tools
git add internal/oracleprobe/stdlib_cases.go internal/oracleprobe/oracleprobe_test.go docs/fixtures/oracle/stdlib/core-regex-oracle.json
git commit -m "test: add regex parity oracle probes"
```

---

### Task 2: Add Apex UTF-16 String Index Helpers

**Files:**
- Create: `/Users/matt/Dev/glade/.worktrees/stdlib-supported/internal/vm/stdlib_apex_string_index.go`
- Create: `/Users/matt/Dev/glade/.worktrees/stdlib-supported/internal/vm/stdlib_apex_string_index_test.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/stdlib-supported/internal/vm/stdlib.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/stdlib-supported/internal/vm/stdlib_string.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/stdlib-supported/internal/vm/regex_test.go`

- [ ] **Step 1: Write failing unit tests for UTF-16 indexing**

Create `internal/vm/stdlib_apex_string_index_test.go`:

```go
package vm

import "testing"

func TestApexUTF16IndexHelpers(t *testing.T) {
	text := "a😀b"
	if got := apexStringLength(text); got != 4 {
		t.Fatalf("apexStringLength(%q) = %d, want 4", text, got)
	}
	start, err := byteIndexForApexStringIndex(text, 1)
	if err != nil || text[start:] != "😀b" {
		t.Fatalf("byteIndexForApexStringIndex start = %d err %v", start, err)
	}
	end, err := byteIndexForApexStringIndex(text, 3)
	if err != nil || text[end:] != "b" {
		t.Fatalf("byteIndexForApexStringIndex end = %d err %v", end, err)
	}
	if _, err := byteIndexForApexStringIndex(text, 2); err == nil {
		t.Fatal("byteIndexForApexStringIndex accepted a split surrogate position")
	}
	if got, err := apexStringIndexForByteIndex(text, end); err != nil || got != 3 {
		t.Fatalf("apexStringIndexForByteIndex = %d err %v, want 3", got, err)
	}
}

func TestApexStringFromCharArrayUsesUTF16Units(t *testing.T) {
	got, err := apexStringFromCharArray(List(Int(55357), Int(56832)).List)
	if err != nil {
		t.Fatal(err)
	}
	if got != "😀" || apexStringLength(got) != 2 {
		t.Fatalf("surrogate pair produced %q length %d, want emoji length 2", got, apexStringLength(got))
	}
	truncated, err := apexStringFromCharArray(List(Int(128512)).List)
	if err != nil {
		t.Fatal(err)
	}
	if apexStringLength(truncated) != 1 {
		t.Fatalf("scalar input length = %d, want one UTF-16 unit", apexStringLength(truncated))
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported
go test ./internal/vm -run 'TestApexUTF16IndexHelpers|TestApexStringFromCharArrayUsesUTF16Units' -count=1
```

Expected: FAIL because helpers do not exist.

- [ ] **Step 3: Add UTF-16 helpers**

Create `internal/vm/stdlib_apex_string_index.go`:

```go
package vm

import (
	"fmt"
	"strings"
	"unicode/utf16"
)

func apexStringLength(text string) int {
	count := 0
	for _, r := range text {
		if r <= 0xffff {
			count++
		} else {
			count += 2
		}
	}
	return count
}

func byteIndexForApexStringIndex(input string, unitIndex int) (int, error) {
	if unitIndex < 0 {
		return 0, fmt.Errorf("string index must be non-negative")
	}
	units := 0
	for byteIndex, r := range input {
		if units == unitIndex {
			return byteIndex, nil
		}
		width := 1
		if r > 0xffff {
			width = 2
		}
		if unitIndex > units && unitIndex < units+width {
			return 0, fmt.Errorf("string index splits a surrogate pair")
		}
		units += width
	}
	if units == unitIndex {
		return len(input), nil
	}
	return 0, fmt.Errorf("string index out of range")
}

func apexStringIndexForByteIndex(input string, targetByte int) (int, error) {
	if targetByte < 0 || targetByte > len(input) {
		return 0, fmt.Errorf("byte index out of range")
	}
	units := 0
	for byteIndex, r := range input {
		if byteIndex == targetByte {
			return units, nil
		}
		if byteIndex > targetByte {
			return 0, fmt.Errorf("byte index is not a rune boundary")
		}
		if r <= 0xffff {
			units++
		} else {
			units += 2
		}
	}
	if targetByte == len(input) {
		return units, nil
	}
	return 0, fmt.Errorf("byte index is not a rune boundary")
}

func apexStringFromCharArray(values []Value) (string, error) {
	units := make([]uint16, 0, len(values))
	for _, item := range values {
		if item.Kind != ValueInt || item.Int < 0 {
			return "", fmt.Errorf("String.fromCharArray expects non-negative Integer UTF-16 units")
		}
		units = append(units, uint16(item.Int&0xffff))
	}
	var b strings.Builder
	for _, r := range utf16.Decode(units) {
		b.WriteRune(r)
	}
	return b.String(), nil
}
```

- [ ] **Step 4: Wire String.length and String.fromCharArray**

In `internal/vm/stdlib_string.go`, change `String.length`:

```go
return Int(int64(apexStringLength(receiver.Text))), true, nil
```

In `internal/vm/stdlib.go`, change `String.fromCharArray`:

```go
case "String.fromCharArray":
	if len(args) != 1 || args[0].Kind != ValueList {
		return Null, fmt.Errorf("String.fromCharArray expects List<Integer>")
	}
	text, err := apexStringFromCharArray(args[0].List)
	if err != nil {
		return Null, err
	}
	return String(text), nil
```

- [ ] **Step 5: Run focused tests**

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported
go test ./internal/vm -run 'TestApexUTF16IndexHelpers|TestApexStringFromCharArrayUsesUTF16Units|TestExecStringStaticFromCharArray' -count=1
```

Expected: PASS after updating any old `String.fromCharArray(new List<Integer>{128512})` expectations to match Salesforce truncation behavior.

- [ ] **Step 6: Commit UTF-16 foundation**

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported
git add internal/vm/stdlib_apex_string_index.go internal/vm/stdlib_apex_string_index_test.go internal/vm/stdlib.go internal/vm/stdlib_string.go internal/vm/regex_test.go internal/vm/stdlib_test.go
git commit -m "fix: align Apex UTF-16 string indices"
```

---

### Task 3: Implement Java Character-Class Algebra Lowering

**Files:**
- Create: `/Users/matt/Dev/glade/.worktrees/stdlib-supported/internal/vm/stdlib_regex_class.go`
- Create: `/Users/matt/Dev/glade/.worktrees/stdlib-supported/internal/vm/stdlib_regex_class_test.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/stdlib-supported/internal/vm/stdlib_regex_engine.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/stdlib-supported/internal/vm/regex_test.go`

- [ ] **Step 1: Write failing lowerer tests**

Create `internal/vm/stdlib_regex_class_test.go`:

```go
package vm

import "testing"

func TestJavaClassAlgebraLowering(t *testing.T) {
	tests := []struct {
		name   string
		source string
		input  string
		want   bool
	}{
		{name: "chain accepts overlap", source: `[a-z&&[m-p]&&[n-o]]+`, input: "no", want: true},
		{name: "chain rejects outside final overlap", source: `[a-z&&[m-p]&&[n-o]]+`, input: "mp", want: false},
		{name: "positive negative accepts", source: `[a-z&&[m-p]&&[^o]]+`, input: "mnp", want: true},
		{name: "positive negative rejects", source: `[a-z&&[m-p]&&[^o]]+`, input: "o", want: false},
		{name: "nested accepts", source: `[a-z&&[m-p&&[^o]]]+`, input: "mnp", want: true},
		{name: "nested rejects", source: `[a-z&&[m-p&&[^o]]]+`, input: "o", want: false},
		{name: "property intersection", source: `[\\p{L}&&[\\p{Ll}]&&[^x]]+`, input: "abcé", want: true},
		{name: "property intersection rejects", source: `[\\p{L}&&[\\p{Ll}]&&[^x]]+`, input: "x", want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, re, err := compileRegexp2Pattern("Pattern.compile", tc.source, 0)
			if err != nil {
				t.Fatal(err)
			}
			match, err := re.FindStringMatchStartingAt(tc.input, 0)
			if err != nil {
				t.Fatal(err)
			}
			got := match != nil && match.Index == 0 && match.Length == len([]rune(tc.input))
			if got != tc.want {
				t.Fatalf("match = %v, want %v", got, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported
go test ./internal/vm -run TestJavaClassAlgebraLowering -count=1
```

Expected: FAIL with `Java regex character-class intersections`.

- [ ] **Step 3: Add the class parser and predicate lowering**

Create `internal/vm/stdlib_regex_class.go` with these public helpers inside package `vm`:

```go
package vm

import (
	"fmt"
	"strings"
)

type javaClassExpr interface {
	javaClassPredicate() string
}

type javaClassSimple struct {
	body string
}

type javaClassIntersection struct {
	operands []javaClassExpr
}

type javaClassNegation struct {
	inner javaClassExpr
}

func rewriteJavaClassAlgebraForRegexp2(source string) (string, error) {
	if !strings.Contains(source, "&&") {
		return source, nil
	}
	var out strings.Builder
	for i := 0; i < len(source); {
		if source[i] != '[' || isEscapedRegexByte(source, i) {
			out.WriteByte(source[i])
			i++
			continue
		}
		end := javaRegexCharClassEnd(source, i)
		if end < 0 {
			out.WriteByte(source[i])
			i++
			continue
		}
		body := source[i+1 : end]
		if !strings.Contains(body, "&&") {
			out.WriteString(source[i : end+1])
			i = end + 1
			continue
		}
		expr, err := parseJavaClassExpr(body)
		if err != nil {
			return "", err
		}
		out.WriteString(javaClassAtom(expr))
		i = end + 1
	}
	return out.String(), nil
}

func javaClassAtom(expr javaClassExpr) string {
	return "(?:" + expr.javaClassPredicate() + `[\s\S]` + ")"
}
```

Fill in `parseJavaClassExpr`, `splitJavaClassIntersectionOperands`, and `javaClassIntersectionOperand` by moving the existing helpers from `stdlib_regex_engine.go` and broadening them:

- A body with leading `^` becomes `javaClassNegation{inner: parse(rest)}`.
- Top-level `&&` splits into `javaClassIntersection`.
- A bracketed operand like `[m-p&&[^o]]` recursively parses its body.
- A simple operand preserves its class body after existing Java escape, Unicode alias, and shorthand rewrites.
- `javaClassSimple.javaClassPredicate()` returns `"(?=[" + body + "])"`.
- `javaClassIntersection.javaClassPredicate()` concatenates operand predicates.
- `javaClassNegation.javaClassPredicate()` returns `"(?!" + javaClassAtom(inner) + ")"`.

- [ ] **Step 4: Replace the old converter**

In `internal/vm/stdlib_regex_engine.go`, replace this call:

```go
regexp2Source, err = javaClassIntersectionsToRegexp2(converted)
```

with this call after Java escape, Unicode alias, and shorthand rewrites:

```go
regexp2Source, err = rewriteJavaClassAlgebraForRegexp2(regexp2Source)
```

The intended order is:

```go
regexp2Source := source
if flags&patternFlagLiteral != 0 {
	regexp2Source = regexp.QuoteMeta(source)
} else {
	converted, err := javaRegexQuoteEscapesToGo(source)
	if err != nil {
		return "", unsupportedCallError(callee + " " + err.Error())
	}
	regexp2Source = converted
	unicodeCharacterClass := flags&patternFlagUnicodeCharacterClass != 0
	regexp2Source, unicodeCharacterClass = rewriteInlineUnicodeCharacterClassFlagForRegexp2(regexp2Source, unicodeCharacterClass)
	regexp2Source = rewriteJavaRegexEscapesForRegexp2(regexp2Source)
	regexp2Source = rewriteJavaUnicodeClassesForRegexp2(regexp2Source)
	regexp2Source = rewriteJavaShorthandClassesForRegexp2(regexp2Source, unicodeCharacterClass)
	regexp2Source, err = rewriteJavaClassAlgebraForRegexp2(regexp2Source)
	if err != nil {
		return "", unsupportedCallError(callee + " " + err.Error())
	}
	regexp2Source = rewriteSimplePossessiveQuantifiersForRegexp2(regexp2Source)
	if feature := unsupportedRegexp2JavaRegexFeature(regexp2Source); feature != "" {
		return "", unsupportedCallError(callee + " " + feature)
	}
}
```

- [ ] **Step 5: Replace unsupported Apex tests with support tests**

In `internal/vm/regex_test.go`, replace `TestExecPatternRejectsUnsupportedClassIntersectionShapes` with:

```go
func TestExecPatternSupportsNestedJavaClassIntersectionShapes(t *testing.T) {
	program, err := CompileAnonymous(`
System.assert(Pattern.matches('[a-z&&[m-p]&&[n-o]]+', 'no'));
System.assert(!Pattern.matches('[a-z&&[m-p]&&[n-o]]+', 'mp'));
System.assert(Pattern.matches('[a-z&&[m-p]&&[^o]]+', 'mnp'));
System.assert(!Pattern.matches('[a-z&&[m-p]&&[^o]]+', 'o'));
System.assert(Pattern.matches('[a-z&&[m-p&&[^o]]]+', 'mnp'));
System.assert(!Pattern.matches('[a-z&&[m-p&&[^o]]]+', 'o'));
System.assert(Pattern.matches('[\\p{L}&&[\\p{Ll}]&&[^x]]+', 'abcé'));
System.assert(!Pattern.matches('[\\p{L}&&[\\p{Ll}]&&[^x]]+', 'x'));
List<String> split = 'amnoq'.split('[a-z&&[m-p]&&[^o]]', -1);
System.assertEquals(3, split.size());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 6: Run class-algebra tests**

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported
go test ./internal/vm -run 'TestJavaClassAlgebraLowering|TestExecPatternSupportsJavaLookaroundNamedGroupsAndClassIntersection|TestExecPatternSupportsNestedJavaClassIntersectionShapes' -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit class algebra**

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported
git add internal/vm/stdlib_regex_class.go internal/vm/stdlib_regex_class_test.go internal/vm/stdlib_regex_engine.go internal/vm/regex_test.go
git commit -m "fix: support Java regex class algebra"
```

---

### Task 4: Add Grapheme Boundary Tables And Input-Aware Regex Plans

**Files:**
- Create: `/Users/matt/Dev/glade/.worktrees/stdlib-supported/internal/vm/stdlib_regex_grapheme.go`
- Create: `/Users/matt/Dev/glade/.worktrees/stdlib-supported/internal/vm/stdlib_regex_grapheme_test.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/stdlib-supported/internal/vm/stdlib_regex_engine.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/stdlib-supported/go.mod`
- Modify: `/Users/matt/Dev/glade/.worktrees/stdlib-supported/go.sum`

- [ ] **Step 1: Add uniseg**

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported
go get github.com/rivo/uniseg@v0.4.7
```

Expected: `go.mod` and `go.sum` change.

- [ ] **Step 2: Write failing grapheme tests**

Create `internal/vm/stdlib_regex_grapheme_test.go`:

```go
package vm

import "testing"

func TestGraphemeBoundaryTable(t *testing.T) {
	text := "e\u0301x"
	table := buildGraphemeBoundaryTable(text)
	if !table.isBoundaryByte(0) || !table.isBoundaryByte(len("e\u0301")) || !table.isBoundaryByte(len(text)) {
		t.Fatalf("missing expected byte boundaries: %#v", table)
	}
	if table.isBoundaryByte(len("e")) {
		t.Fatalf("boundary table split combining sequence")
	}
}

func TestCompileGraphemeRegexPlanMatchesExtendedClusters(t *testing.T) {
	text := "👍🏽x"
	plan, err := compileRegexp2PlanForInput("Pattern.compile", `\X`, 0, text)
	if err != nil {
		t.Fatal(err)
	}
	match, err := plan.findValidStartingAt(text, 0)
	if err != nil {
		t.Fatal(err)
	}
	if match == nil {
		t.Fatal("expected grapheme match")
	}
	indices, err := plan.matchByteIndices(text, match, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := text[indices[0]:indices[1]]; got != "👍🏽" {
		t.Fatalf("group = %q, want thumbs-up skin-tone cluster", got)
	}
}
```

- [ ] **Step 3: Run the test to verify it fails**

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported
go test ./internal/vm -run 'TestGraphemeBoundaryTable|TestCompileGraphemeRegexPlanMatchesExtendedClusters' -count=1
```

Expected: FAIL because the helpers do not exist.

- [ ] **Step 4: Add grapheme plan helpers**

Create `internal/vm/stdlib_regex_grapheme.go` with this shape:

```go
package vm

import (
	"regexp"
	"sort"
	"strings"

	"github.com/dlclark/regexp2"
	"github.com/rivo/uniseg"
)

type graphemeCluster struct {
	text      string
	startByte int
	endByte   int
}

type graphemeBoundaryTable struct {
	clusters       []graphemeCluster
	boundaryBytes  map[int]bool
}

type regexp2Plan struct {
	source             string
	re                 *regexp2.Regexp
	grapheme          *graphemeBoundaryTable
	internalGroupNames map[string]bool
	publicGroupNumbers []int
}

func buildGraphemeBoundaryTable(input string) *graphemeBoundaryTable {
	table := &graphemeBoundaryTable{boundaryBytes: map[int]bool{0: true}}
	pos := 0
	g := uniseg.NewGraphemes(input)
	for g.Next() {
		cluster := g.Str()
		next := pos + len(cluster)
		table.clusters = append(table.clusters, graphemeCluster{text: cluster, startByte: pos, endByte: next})
		table.boundaryBytes[next] = true
		pos = next
	}
	table.boundaryBytes[len(input)] = true
	return table
}

func (g *graphemeBoundaryTable) isBoundaryByte(pos int) bool {
	if g == nil {
		return true
	}
	return g.boundaryBytes[pos]
}

func compileRegexp2PlanForInput(callee, source string, flags int64, input string) (*regexp2Plan, error) {
	regexp2Source, err := compileRegexp2Source(callee, source, flags)
	if err != nil {
		return nil, err
	}
	table := buildGraphemeBoundaryTable(input)
	internal := map[string]bool{}
	regexp2Source = rewriteGraphemeTokensForRegexp2(regexp2Source, table, internal)
	re, err := regexp2.Compile(regexp2Source, regexp2.None)
	if err != nil {
		return nil, newPatternSyntaxExceptionError(source, err)
	}
	re.MatchTimeout = regexp2MatchTimeout
	return &regexp2Plan{source: regexp2Source, re: re, grapheme: table, internalGroupNames: internal, publicGroupNumbers: regexp2PublicGroupNumbers(re, internal)}, nil
}
```

Implement `rewriteGraphemeTokensForRegexp2` with these rules:

- Scan outside character classes.
- Replace `\X` with `(?<__gladeGX0>(?:clusterAlternation))`.
- Replace `\b{g}` with `(?<__gladeGB0>)`.
- Increment the suffix for each internal group name.
- If the user pattern already contains a name with the `__gladeG` prefix, start internal suffixes above the highest conflicting suffix so public named groups are not overwritten.
- Build cluster alternation from the current input, deduped and sorted by byte length descending.
- Use `regexp.QuoteMeta(cluster)` for each literal cluster.
- Use `(?!)` when `\X` appears and the input has no clusters.

Add public group mapping so internal validation captures do not leak through `groupCount`, `group(int)`, replacement parsing, or saved group indices:

```go
func regexp2PublicGroupNumbers(re *regexp2.Regexp, internal map[string]bool) []int {
	var out []int
	for _, number := range re.GetGroupNumbers() {
		name := re.GroupNameFromNumber(number)
		if internal[name] {
			continue
		}
		out = append(out, number)
	}
	return out
}
```

`plan.matchByteIndices` must iterate over `publicGroupNumbers`, fetch each group by regexp2 group number, and append only public byte bounds to the stored matcher group list.

Implement match validation:

```go
func (p *regexp2Plan) validGraphemeMatch(input string, match *regexp2.Match) bool {
	if p.grapheme == nil {
		return true
	}
	for name := range p.internalGroupNames {
		group := match.GroupByName(name)
		if group == nil {
			continue
		}
		for _, capture := range group.Captures {
			startByte, err := byteIndexForRuneIndex(input, capture.Index)
			if err != nil {
				return false
			}
			endByte, err := byteIndexForRuneIndex(input, capture.Index+capture.Length)
			if err != nil {
				return false
			}
			if !p.grapheme.isBoundaryByte(startByte) || !p.grapheme.isBoundaryByte(endByte) {
				return false
			}
			if strings.HasPrefix(name, "__gladeGX") && startByte == endByte {
				return false
			}
		}
	}
	return true
}
```

Implement `findValidStartingAt` by calling `FindStringMatchStartingAt` and `FindNextMatch` until `validGraphemeMatch` accepts the match or the engine returns nil.

- [ ] **Step 5: Stop rewriting `\X` to the combining-mark approximation**

In `rewriteJavaRegexEscapesForRegexp2`, remove this old branch:

```go
case 'X':
	if inClass {
		out.WriteString(`X`)
	} else {
		out.WriteString(`(?:\P{M}\p{M}*|\p{M}+)`)
	}
	i++
```

Replace it with:

```go
case 'X':
	out.WriteByte(ch)
	i++
	out.WriteByte(source[i])
```

- [ ] **Step 6: Run grapheme helper tests**

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported
go test ./internal/vm -run 'TestGraphemeBoundaryTable|TestCompileGraphemeRegexPlanMatchesExtendedClusters' -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit grapheme helpers**

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported
git add go.mod go.sum internal/vm/stdlib_regex_grapheme.go internal/vm/stdlib_regex_grapheme_test.go internal/vm/stdlib_regex_engine.go
git commit -m "fix: add grapheme-aware regex planning"
```

---

### Task 5: Wire Matcher, Pattern, Replacement, And Split APIs

**Files:**
- Modify: `/Users/matt/Dev/glade/.worktrees/stdlib-supported/internal/vm/stdlib.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/stdlib-supported/internal/vm/stdlib_regex.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/stdlib-supported/internal/vm/stdlib_regex_engine.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/stdlib-supported/internal/vm/regex_test.go`

- [ ] **Step 1: Write Apex-level tests for public behavior**

Add this test to `internal/vm/regex_test.go`:

```go
func TestExecPatternSupportsExtendedGraphemeClustersAndBoundaries(t *testing.T) {
	program, err := CompileAnonymous(`
String gx = String.fromCharArray(new List<Integer>{92}) + 'X';
String bg = String.fromCharArray(new List<Integer>{92}) + 'b{g}';
String crlf = String.fromCharArray(new List<Integer>{13,10});
String jamo = String.fromCharArray(new List<Integer>{4352,4449});
String flagUS = String.fromCharArray(new List<Integer>{55356,56826,55356,56824});
String thumbTone = String.fromCharArray(new List<Integer>{55357,56397,55356,57341});
String family = String.fromCharArray(new List<Integer>{55357,56424,8205,55357,56425,8205,55357,56423,8205,55357,56422});
String mark = String.fromCharArray(new List<Integer>{769});
String combining = 'e' + mark;

System.assert(Pattern.matches(gx, crlf));
System.assert(Pattern.matches(gx, jamo));
System.assert(Pattern.matches(gx, flagUS));
System.assert(Pattern.matches(gx, thumbTone));
System.assert(Pattern.matches(gx, family));
System.assert(Pattern.matches(gx, mark));
System.assert(Pattern.matches(gx, combining));

Matcher thumb = Pattern.compile(gx).matcher(thumbTone + 'x');
System.assert(thumb.find());
System.assertEquals(thumbTone, thumb.group());
System.assertEquals(0, thumb.start());
System.assertEquals(4, thumb.end());
System.assert(thumb.find());
System.assertEquals('x', thumb.group());
System.assertEquals(4, thumb.start());
System.assertEquals(5, thumb.end());

Matcher boundary = Pattern.compile(bg).matcher(combining + 'x');
System.assert(boundary.find());
System.assertEquals(0, boundary.start());
System.assert(boundary.find());
System.assertEquals(2, boundary.start());
System.assert(boundary.find());
System.assertEquals(3, boundary.start());
System.assert(!boundary.find());

List<String> parts = (thumbTone + 'x').split(gx, -1);
System.assertEquals(3, parts.size());
System.assertEquals('', parts[0]);
System.assertEquals('', parts[1]);
System.assertEquals('', parts[2]);

System.assertEquals('Qx', Pattern.compile(gx).matcher(thumbTone + 'x').replaceFirst('Q'));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 2: Run the public behavior test to verify it fails**

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported
go test ./internal/vm -run TestExecPatternSupportsExtendedGraphemeClustersAndBoundaries -count=1
```

Expected: FAIL until API routing uses grapheme plans and UTF-16 spans.

- [ ] **Step 3: Convert matcher regions and groups to Apex indices**

In `internal/vm/stdlib_regex.go`, change every region and span conversion to use the new helpers:

```go
matcher.Fields["regionEnd"] = Int(int64(apexStringLength(args[0].Text)))
```

Use `byteIndexForApexStringIndex` for `Matcher.find(start)` and `Matcher.region(start, end)`.

Use `apexStringIndexForByteIndex` in `matcherGroupBounds`.

Keep stored group bounds as bytes. Only API inputs and outputs use Apex UTF-16 indices.

- [ ] **Step 4: Route Pattern.matches through input-aware compile**

In `internal/vm/stdlib.go`, replace `compileRegexp2Pattern("Pattern.matches", pattern, 0)` with:

```go
plan, err := compileRegexp2PlanForInput("Pattern.matches", pattern, 0, input)
if err != nil {
	return Null, err
}
match, err := plan.findValidStartingAt(input, 0)
```

Keep the whole-string check, but compare against `utf8.RuneCountInString(input)` because regexp2 reports rune positions internally.

- [ ] **Step 5: Route Matcher operations through input-aware plans**

In `internal/vm/stdlib_regex_engine.go`, change `matcherRegexp2MatchIndices` and `matcherRegexp2FindIndices` to compile a `regexp2Plan` against the actual text searched:

```go
plan, err := matcherRegexp2PlanForInput(matcher, text)
if err != nil {
	return nil, err
}
match, err := plan.findValidStartingAt(text, startAt)
```

Use `plan.matchByteIndices(input, match, runeOffset)` so internal grapheme groups are removed from public group storage.

- [ ] **Step 6: Route replacement and split through valid-match loops**

In `matcherReplaceRegexp2`, accept the matcher or source metadata instead of only the rewritten string source:

```go
func matcherReplaceRegexp2(name string, matcher Value, input string, region matcherRegionBounds, args []Value, all bool) (string, error)
```

Compile the plan against `regionText`, parse replacement with `len(plan.publicGroupNumbers)-1`, and iterate with `plan.findValidStartingAt`.

In `splitRegexRegexp2`, compile a plan with the full input and skip invalid internal grapheme matches the same way `find` does.

- [ ] **Step 7: Keep Pattern.split from double-compiling old rewritten source**

In `callPatternMember`, change the split branch to pass the Pattern object:

```go
parts, err := patternSplit(receiver, args)
```

Implement:

```go
func patternSplit(pattern Value, args []Value) ([]string, error) {
	if len(args) != 1 && len(args) != 2 {
		return nil, fmt.Errorf("Pattern.split expects input String and optional Integer limit")
	}
	input, err := stringArg("Pattern.split", args[:1])
	if err != nil {
		return nil, err
	}
	limit := int64(0)
	if len(args) == 2 {
		if args[1].Kind != ValueInt {
			return nil, fmt.Errorf("Pattern.split expects input String and optional Integer limit")
		}
		limit = args[1].Int
	}
	source, err := patternSourceOnly(pattern)
	if err != nil {
		return nil, err
	}
	flags := patternFlags(pattern)
	return splitRegexWithFlags("Pattern.split", source, flags, input, limit)
}
```

- [ ] **Step 8: Run focused API tests**

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported
go test ./internal/vm -run 'TestExecPatternSupportsExtendedGraphemeClustersAndBoundaries|TestExecPatternSupportsJavaGraphemeMatcherForCombiningMarks|TestExecMatcherReplacementUsesRegexp2Features|TestExecPatternSupportsNestedJavaClassIntersectionShapes' -count=1
```

Expected: PASS.

- [ ] **Step 9: Commit API routing**

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported
git add internal/vm/stdlib.go internal/vm/stdlib_regex.go internal/vm/stdlib_regex_engine.go internal/vm/regex_test.go
git commit -m "fix: route regex APIs through parity engine"
```

---

### Task 6: Update Fixtures, Coverage Notes, And Support Status

**Files:**
- Modify: `/Users/matt/Dev/glade/.worktrees/stdlib-supported-tools/docs/fixtures/*.json`
- Modify: `/Users/matt/Dev/glade/.worktrees/stdlib-supported-tools/internal/capability/stdlib.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/stdlib-supported/docs/STDLIB_COVERAGE.md`

- [ ] **Step 1: Rename unsupported fixtures to supported fixtures**

In the tools worktree, rename old unsupported regex fixtures:

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported-tools
git mv docs/fixtures/core-pattern-class-intersection-unsupported.json docs/fixtures/core-pattern-class-intersection-stdlib.json
git mv docs/fixtures/core-pattern-split-nullable-unsupported.json docs/fixtures/core-pattern-split-nullable-stdlib.json
```

If a file has already been renamed in the current branch, leave it in place and update its contents.

- [ ] **Step 2: Add grapheme fixture coverage**

Create `docs/fixtures/core-pattern-grapheme-stdlib.json`:

```json
{
  "name": "core-pattern-grapheme-stdlib",
  "evidence": [
    {
      "surfaceId": "apex:System.Pattern.matches(String,String)",
      "symbol": "Pattern.matches",
      "kind": "exec",
      "notes": "Extended grapheme cluster matching covers CRLF, Hangul Jamo, regional indicators, emoji modifiers, ZWJ sequences, combining marks, and isolated marks."
    },
    {
      "symbol": "Matcher.find",
      "kind": "exec",
      "notes": "Matcher.find honors UAX #29 grapheme clusters and grapheme boundaries."
    },
    {
      "surfaceId": "apex:System.Matcher.start()",
      "symbol": "Matcher.start",
      "kind": "exec",
      "notes": "Matcher.start returns Apex UTF-16 code-unit indices."
    },
    {
      "surfaceId": "apex:System.Matcher.end()",
      "symbol": "Matcher.end",
      "kind": "exec",
      "notes": "Matcher.end returns Apex UTF-16 code-unit indices."
    },
    {
      "surfaceId": "apex:System.Pattern.split(String,Integer)",
      "symbol": "Pattern.split",
      "kind": "exec",
      "notes": "Pattern.split handles grapheme cluster delimiters with Java trailing-empty behavior."
    },
    {
      "surfaceId": "apex:System.String.split(String,Integer)",
      "symbol": "String.split",
      "kind": "exec",
      "notes": "String.split handles grapheme cluster delimiters with Java trailing-empty behavior."
    },
    {
      "symbol": "Matcher.replaceFirst",
      "kind": "exec",
      "notes": "Replacement uses the same validated grapheme-aware match path."
    }
  ],
  "source": [
    {
      "path": "anonymous.apex",
      "content": "String gx = String.fromCharArray(new List<Integer>{92}) + 'X';\nString bg = String.fromCharArray(new List<Integer>{92}) + 'b{g}';\nString crlf = String.fromCharArray(new List<Integer>{13,10});\nString jamo = String.fromCharArray(new List<Integer>{4352,4449});\nString flagUS = String.fromCharArray(new List<Integer>{55356,56826,55356,56824});\nString thumbTone = String.fromCharArray(new List<Integer>{55357,56397,55356,57341});\nString family = String.fromCharArray(new List<Integer>{55357,56424,8205,55357,56425,8205,55357,56423,8205,55357,56422});\nString mark = String.fromCharArray(new List<Integer>{769});\nString combining = 'e' + mark;\nSystem.assert(Pattern.matches(gx, crlf));\nSystem.assert(Pattern.matches(gx, jamo));\nSystem.assert(Pattern.matches(gx, flagUS));\nSystem.assert(Pattern.matches(gx, thumbTone));\nSystem.assert(Pattern.matches(gx, family));\nSystem.assert(Pattern.matches(gx, mark));\nSystem.assert(Pattern.matches(gx, combining));\nMatcher thumb = Pattern.compile(gx).matcher(thumbTone + 'x');\nSystem.assert(thumb.find());\nSystem.assertEquals(thumbTone, thumb.group());\nSystem.assertEquals(0, thumb.start());\nSystem.assertEquals(4, thumb.end());\nSystem.assert(thumb.find());\nSystem.assertEquals('x', thumb.group());\nSystem.assertEquals(4, thumb.start());\nSystem.assertEquals(5, thumb.end());\nMatcher boundary = Pattern.compile(bg).matcher(combining + 'x');\nSystem.assert(boundary.find());\nSystem.assertEquals(0, boundary.start());\nSystem.assert(boundary.find());\nSystem.assertEquals(2, boundary.start());\nSystem.assert(boundary.find());\nSystem.assertEquals(3, boundary.start());\nSystem.assert(!boundary.find());\nList<String> stringParts = (thumbTone + 'x').split(gx, -1);\nSystem.assertEquals(3, stringParts.size());\nSystem.assertEquals('', stringParts[0]);\nSystem.assertEquals('', stringParts[1]);\nSystem.assertEquals('', stringParts[2]);\nList<String> patternParts = Pattern.compile(gx).split(thumbTone + 'x', -1);\nSystem.assertEquals(3, patternParts.size());\nSystem.assertEquals('', patternParts[0]);\nSystem.assertEquals('', patternParts[1]);\nSystem.assertEquals('', patternParts[2]);\nSystem.assertEquals('Qx', Pattern.compile(gx).matcher(thumbTone + 'x').replaceFirst('Q'));\n"
    }
  ],
  "command": {
    "kind": "exec",
    "args": [
      "String gx = String.fromCharArray(new List<Integer>{92}) + 'X';\nString bg = String.fromCharArray(new List<Integer>{92}) + 'b{g}';\nString crlf = String.fromCharArray(new List<Integer>{13,10});\nString jamo = String.fromCharArray(new List<Integer>{4352,4449});\nString flagUS = String.fromCharArray(new List<Integer>{55356,56826,55356,56824});\nString thumbTone = String.fromCharArray(new List<Integer>{55357,56397,55356,57341});\nString family = String.fromCharArray(new List<Integer>{55357,56424,8205,55357,56425,8205,55357,56423,8205,55357,56422});\nString mark = String.fromCharArray(new List<Integer>{769});\nString combining = 'e' + mark;\nSystem.assert(Pattern.matches(gx, crlf));\nSystem.assert(Pattern.matches(gx, jamo));\nSystem.assert(Pattern.matches(gx, flagUS));\nSystem.assert(Pattern.matches(gx, thumbTone));\nSystem.assert(Pattern.matches(gx, family));\nSystem.assert(Pattern.matches(gx, mark));\nSystem.assert(Pattern.matches(gx, combining));\nMatcher thumb = Pattern.compile(gx).matcher(thumbTone + 'x');\nSystem.assert(thumb.find());\nSystem.assertEquals(thumbTone, thumb.group());\nSystem.assertEquals(0, thumb.start());\nSystem.assertEquals(4, thumb.end());\nSystem.assert(thumb.find());\nSystem.assertEquals('x', thumb.group());\nSystem.assertEquals(4, thumb.start());\nSystem.assertEquals(5, thumb.end());\nMatcher boundary = Pattern.compile(bg).matcher(combining + 'x');\nSystem.assert(boundary.find());\nSystem.assertEquals(0, boundary.start());\nSystem.assert(boundary.find());\nSystem.assertEquals(2, boundary.start());\nSystem.assert(boundary.find());\nSystem.assertEquals(3, boundary.start());\nSystem.assert(!boundary.find());\nList<String> stringParts = (thumbTone + 'x').split(gx, -1);\nSystem.assertEquals(3, stringParts.size());\nSystem.assertEquals('', stringParts[0]);\nSystem.assertEquals('', stringParts[1]);\nSystem.assertEquals('', stringParts[2]);\nList<String> patternParts = Pattern.compile(gx).split(thumbTone + 'x', -1);\nSystem.assertEquals(3, patternParts.size());\nSystem.assertEquals('', patternParts[0]);\nSystem.assertEquals('', patternParts[1]);\nSystem.assertEquals('', patternParts[2]);\nSystem.assertEquals('Qx', Pattern.compile(gx).matcher(thumbTone + 'x').replaceFirst('Q'));\n"
    ]
  },
  "expected": {
    "result": {
      "debug": null,
      "ok": true
    }
  }
}
```

- [ ] **Step 3: Update generated support notes**

In `internal/capability/stdlib.go`, remove these fences from Pattern and String.split rows once product fixtures pass:

```text
full UAX #29 grapheme parity
general nested class-intersection algebra
remaining limits follow the shared Pattern regex fences
```

Keep any remaining honest fences for APIs not covered by this plan, such as unimplemented `Matcher.appendReplacement` and `Matcher.appendTail`.

- [ ] **Step 4: Regenerate support docs**

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported-tools
go run ./cmd/glade-tools stdlib --output ../stdlib-supported/docs/STDLIB_COVERAGE.md
go run ./cmd/glade-tools stub-contracts --output docs/generated/stubs/STUB_CONTRACTS.json
```

- [ ] **Step 5: Run fixture validation**

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported-tools
go run ./cmd/glade-tools validate docs/fixtures/core-pattern-class-intersection-stdlib.json
go run ./cmd/glade-tools validate docs/fixtures/core-pattern-grapheme-stdlib.json
go run ./cmd/glade-tools run docs/fixtures/core-pattern-class-intersection-stdlib.json --glade ../stdlib-supported/glade
go run ./cmd/glade-tools run docs/fixtures/core-pattern-grapheme-stdlib.json --glade ../stdlib-supported/glade
```

Expected: all commands exit `0`.

- [ ] **Step 6: Commit docs and fixtures**

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported-tools
git add docs/fixtures docs/generated/stubs/STUB_CONTRACTS.json internal/capability/stdlib.go
git commit -m "docs: mark regex parity fixtures supported"

cd /Users/matt/Dev/glade/.worktrees/stdlib-supported
git add docs/STDLIB_COVERAGE.md
git commit -m "docs: update regex support coverage"
```

---

### Task 7: Full Verification And Merge Prep

**Files:**
- Product worktree: all changed product files.
- Tools worktree: all changed tool and fixture files.

- [ ] **Step 1: Run focused product tests**

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported
go test ./internal/vm -count=1
```

Expected: PASS.

- [ ] **Step 2: Run full product tests**

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported
go test ./...
```

Expected: PASS. If DAP timeout flakes appear, rerun the failed package once and record the flake in the handoff.

- [ ] **Step 3: Run full tools tests**

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported-tools
go test ./...
```

Expected: PASS.

- [ ] **Step 4: Check generated docs**

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported-tools
go run ./cmd/glade-tools stdlib --check ../stdlib-supported/docs/STDLIB_COVERAGE.md
go run ./cmd/glade-tools stub-contracts --check docs/generated/stubs/STUB_CONTRACTS.json
```

Expected: PASS.

- [ ] **Step 5: Search for stale fences**

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported
rg -n "combining-mark clusters|full UAX #29|general nested class-intersection|Go regexp-backed|Go regexp syntax" docs internal

cd /Users/matt/Dev/glade/.worktrees/stdlib-supported-tools
rg -n "combining-mark clusters|full UAX #29|general nested class-intersection|Go regexp-backed|Go regexp syntax" docs internal
```

Expected: no stale references except historical tests or comments that explain removed behavior.

- [ ] **Step 6: Prepare merge into local main**

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported
git status --short

cd /Users/matt/Dev/glade/.worktrees/stdlib-supported-tools
git status --short
```

Expected: both worktrees are clean after commits. Merge only after review.

---

## Self-Review

- Spec coverage: the plan covers Salesforce oracle capture, UTF-16 matcher positions, Java nested class intersections, UAX #29 grapheme clusters, `\b{g}`, Matcher APIs, Pattern APIs, String.split, replacement, docs, and fixtures.
- Scope guard: `Matcher.appendReplacement` and `Matcher.appendTail` stay out of scope because they require Java StringBuffer append semantics and are not part of the current regex parity gap.
- Risk guard: ordinary regexp2 patterns keep the existing path. Grapheme-specific validation runs only when `\X` or `\b{g}` appears outside character classes.
- Performance guard: parser and boundary helpers get fast unit tests. Full Apex execution tests stay small and oracle-backed.
