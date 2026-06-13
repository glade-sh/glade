# Standard Library Partials To Supported Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move the listed `String`, `Decimal`, `JSON`, `Pattern`/`Matcher`, `EncodingUtil`, and `Crypto.generateDigest` standard-library rows from `partial` to `supported` using `oaer-probe-max` as the Salesforce oracle.

**Architecture:** Add an oracle probe loop in `/Users/matt/Dev/glade-tools` that deploys/runs Apex against `oaer-probe-max`, records exact values and exception shapes, and writes checked evidence fixtures. Then update `/Users/matt/Dev/glade` runtime code until the local VM matches those fixtures. Promote rows in the first-party stdlib catalog only after oracle evidence, local tests, generated docs, and public site wording agree.

**Tech Stack:** Go, Salesforce CLI, Apex execute anonymous, Apex class deploy/test, Glade VM, `glade-tools` compat/catalog generators, VitePress docs.

---

## Current Baseline

Checked on 2026-06-13:

- `/Users/matt/Dev/glade` is on clean local `main`.
- `/Users/matt/Dev/glade-tools` is on clean local `main`.
- `sf org display --target-org oaer-probe-max --json` returns status `0`, username `test-xqyasuqprt8i@example.com`, org id `00DQL00000VntW92AJ`.
- `go run ./cmd/glade-tools stdlib --json` reports these target rows as `partial`:
  - `String.split`
  - `Decimal.round`
  - `Decimal.setScale`
  - `JSON.deserialize`
  - `JSON.deserializeStrict`
  - `JSON.deserializeUntyped`
  - `JSON.serialize`
  - `JSON.serializePretty`
  - `Pattern.compile`
  - `Pattern.matches`
  - `Matcher.find`
  - `Matcher.group`
  - `Matcher.matches`
  - `EncodingUtil.urlDecode`
  - `EncodingUtil.urlEncode`
  - `Crypto.generateDigest`

## Support Rule

A row is `supported` only when all are true:

- A checked oracle fixture captures Salesforce output from `oaer-probe-max`.
- A local Glade fixture or VM test asserts the same value, type, and exception shape.
- The implementation has no row-level local fence for the listed API.
- The stdlib catalog row is `StatusSupported`.
- `docs/STDLIB_COVERAGE.md`, `docs/COMPATIBILITY_DASHBOARD.md`, `docs/KNOWN_GAPS.md`, and `site/docs-src/guide/support-map.md` do not describe the row as partial.

Do not promote a row from local judgment alone. The scratch org is the pencil line.

## File Map

Product repo: `/Users/matt/Dev/glade`

- Modify: `/Users/matt/Dev/glade/internal/vm/value.go` for exact decimal storage if needed.
- Modify: `/Users/matt/Dev/glade/internal/vm/stdlib_number.go` for `Decimal.round` and `Decimal.setScale`.
- Modify: `/Users/matt/Dev/glade/internal/vm/stdlib.go` for shared decimal helpers, `Pattern.compile`, `Pattern.matches`, `String.split`, `EncodingUtil`, and `Crypto.generateDigest`.
- Modify: `/Users/matt/Dev/glade/internal/vm/stdlib_regex.go` for `Pattern` and `Matcher` members.
- Modify: `/Users/matt/Dev/glade/internal/vm/json_runtime.go` for JSON serialize/deserialize and typed coercion.
- Modify: `/Users/matt/Dev/glade/internal/vm/json_parser.go` for parser edge parity where probes show differences.
- Modify: `/Users/matt/Dev/glade/internal/vm/json_generator.go` for generator/pretty output parity where probes show differences.
- Modify: `/Users/matt/Dev/glade/internal/vm/stdlib_test.go`.
- Modify: `/Users/matt/Dev/glade/internal/vm/json_test.go`.
- Modify: `/Users/matt/Dev/glade/internal/vm/regex_test.go`.
- Modify: `/Users/matt/Dev/glade/internal/vm/platform_test.go`.
- Modify generated docs after catalog promotion:
  - `/Users/matt/Dev/glade/docs/STDLIB_COVERAGE.md`
  - `/Users/matt/Dev/glade/docs/COMPATIBILITY_DASHBOARD.md`
  - `/Users/matt/Dev/glade/docs/KNOWN_GAPS.md`
  - `/Users/matt/Dev/glade/site/docs-src/guide/support-map.md`

Tools repo: `/Users/matt/Dev/glade-tools`

- Create: `/Users/matt/Dev/glade-tools/internal/oracleprobe/case.go`
- Create: `/Users/matt/Dev/glade-tools/internal/oracleprobe/render.go`
- Create: `/Users/matt/Dev/glade-tools/internal/oracleprobe/runner.go`
- Create: `/Users/matt/Dev/glade-tools/internal/oracleprobe/stdlib_cases.go`
- Create: `/Users/matt/Dev/glade-tools/internal/oracleprobe/write.go`
- Create: `/Users/matt/Dev/glade-tools/internal/oracleprobe/oracleprobe_test.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/toolcli/cli.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/toolcli/compat_command.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/capability/stdlib.go`
- Create checked oracle output under `/Users/matt/Dev/glade-tools/docs/fixtures/oracle/stdlib/*.json`.

## Parallel Work Shape

Run Task 1 first. It builds the measuring jig. After Task 1, four squads can work in parallel:

- Squad A: Decimal, EncodingUtil, Crypto.
- Squad B: Regex and String split.
- Squad C: JSON parser/generator/untyped.
- Squad D: JSON typed/strict/SObject/DTO.

The catalog/docs promotion waits until all squads land.

---

### Task 0: Create Paired Worktrees And Baseline

**Files:**
- Read: `/Users/matt/Dev/glade/docs/STDLIB_COVERAGE.md`
- Read: `/Users/matt/Dev/glade-tools/internal/capability/stdlib.go`

- [ ] **Step 0.1: Create the product worktree**

Run:

```bash
cd /Users/matt/Dev/glade
git status --short --branch
git worktree add .worktrees/stdlib-supported -b codex/stdlib-supported main
```

Expected:

```text
## main
Preparing worktree (new branch 'codex/stdlib-supported')
```

- [ ] **Step 0.2: Create the tools worktree**

Run:

```bash
cd /Users/matt/Dev/glade-tools
git status --short --branch
git worktree add /Users/matt/Dev/glade/.worktrees/stdlib-supported-tools -b codex/stdlib-supported-tools main
```

Expected:

```text
## main
Preparing worktree (new branch 'codex/stdlib-supported-tools')
```

- [ ] **Step 0.3: Confirm the scratch org**

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported
sf org display --target-org oaer-probe-max --json | jq -r '.status as $s | "status=\($s) username=\(.result.username // "") orgId=\(.result.id // "")"'
```

Expected:

```text
status=0 username=test-xqyasuqprt8i@example.com orgId=00DQL00000VntW92AJ
```

- [ ] **Step 0.4: Capture the current partial rows**

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported-tools
go run ./cmd/glade-tools stdlib --json | jq -r '
  .[] |
  select(.status == "partial" and (
    (.area == "String" and .api == "String.split") or
    (.area == "Decimal" and (.api == "Decimal.round" or .api == "Decimal.setScale")) or
    (.area == "JSON") or
    (.area == "Pattern") or
    (.area == "EncodingUtil") or
    (.area == "Crypto" and .api == "Crypto.generateDigest")
  )) |
  [.area, .api, .notes] | @tsv
'
```

Expected: 16 rows, matching the list in this plan.

### Task 1: Add The Salesforce Oracle Probe Harness

**Files:**
- Create: `/Users/matt/Dev/glade-tools/internal/oracleprobe/case.go`
- Create: `/Users/matt/Dev/glade-tools/internal/oracleprobe/render.go`
- Create: `/Users/matt/Dev/glade-tools/internal/oracleprobe/runner.go`
- Create: `/Users/matt/Dev/glade-tools/internal/oracleprobe/stdlib_cases.go`
- Create: `/Users/matt/Dev/glade-tools/internal/oracleprobe/write.go`
- Create: `/Users/matt/Dev/glade-tools/internal/oracleprobe/oracleprobe_test.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/toolcli/cli.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/toolcli/compat_command.go`

- [ ] **Step 1.1: Add the probe data model**

Create `/Users/matt/Dev/glade-tools/internal/oracleprobe/case.go`:

```go
package oracleprobe

type Mode string

const (
	ModeAnonymous Mode = "anonymous"
	ModeDeploy    Mode = "deploy"
)

type Case struct {
	ID          string `json:"id"`
	Area        string `json:"area"`
	API         string `json:"api"`
	Mode        Mode   `json:"mode"`
	SetupClass  string `json:"setupClass,omitempty"`
	Statements  []string `json:"statements,omitempty"`
	Expression  string `json:"expression"`
	ValueType   string `json:"valueType,omitempty"`
	ExpectThrow bool   `json:"expectThrow,omitempty"`
}

type Result struct {
	ID               string `json:"id"`
	Area             string `json:"area"`
	API              string `json:"api"`
	Mode             Mode   `json:"mode"`
	Value            string `json:"value,omitempty"`
	ValueType        string `json:"valueType,omitempty"`
	ExceptionType    string `json:"exceptionType,omitempty"`
	ExceptionMessage string `json:"exceptionMessage,omitempty"`
	RawLogLine       string `json:"rawLogLine,omitempty"`
}

type Report struct {
	TargetOrg string   `json:"targetOrg"`
	Username  string   `json:"username,omitempty"`
	OrgID     string   `json:"orgId,omitempty"`
	APIVersion string  `json:"apiVersion,omitempty"`
	Results   []Result `json:"results"`
}
```

- [ ] **Step 1.2: Render execute-anonymous Apex**

Create `/Users/matt/Dev/glade-tools/internal/oracleprobe/render.go`:

```go
package oracleprobe

import (
	"fmt"
	"strings"
)

const marker = "GLADE_STDLIB_ORACLE:"

func RenderAnonymous(cases []Case) string {
	var b strings.Builder
	b.WriteString("List<Object> __gladeRows = new List<Object>();\n")
	for _, tc := range cases {
		b.WriteString("try {\n")
		for _, statement := range tc.Statements {
			b.WriteString("  ")
			b.WriteString(statement)
			if !strings.HasSuffix(strings.TrimSpace(statement), ";") {
				b.WriteString(";")
			}
			b.WriteString("\n")
		}
		b.WriteString("  Object __v = ")
		b.WriteString(tc.Expression)
		b.WriteString(";\n")
		b.WriteString(fmt.Sprintf("  __gladeRows.add(new Map<String,Object>{'id'=>'%s','area'=>'%s','api'=>'%s','mode'=>'%s','value'=>String.valueOf(__v),'valueType'=>'%s'});\n",
			escapeApexString(tc.ID), escapeApexString(tc.Area), escapeApexString(tc.API), tc.Mode, escapeApexString(tc.ValueType)))
		b.WriteString("} catch (Exception e) {\n")
		b.WriteString(fmt.Sprintf("  __gladeRows.add(new Map<String,Object>{'id'=>'%s','area'=>'%s','api'=>'%s','mode'=>'%s','exceptionType'=>e.getTypeName(),'exceptionMessage'=>e.getMessage()});\n",
			escapeApexString(tc.ID), escapeApexString(tc.Area), escapeApexString(tc.API), tc.Mode))
		b.WriteString("}\n")
	}
	b.WriteString("System.debug('")
	b.WriteString(marker)
	b.WriteString("' + JSON.serialize(__gladeRows));\n")
	return b.String()
}

func escapeApexString(text string) string {
	text = strings.ReplaceAll(text, "\\", "\\\\")
	return strings.ReplaceAll(text, "'", "\\'")
}
```

- [ ] **Step 1.3: Add runner shell-out**

Create `/Users/matt/Dev/glade-tools/internal/oracleprobe/runner.go`:

```go
package oracleprobe

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Options struct {
	TargetOrg string
	WorkDir   string
}

func RunAnonymous(ctx context.Context, cases []Case, opts Options) (Report, error) {
	if opts.TargetOrg == "" {
		return Report{}, fmt.Errorf("target org is required")
	}
	dir := opts.WorkDir
	if dir == "" {
		tmp, err := os.MkdirTemp("", "glade-stdlib-oracle-*")
		if err != nil {
			return Report{}, err
		}
		dir = tmp
	}
	apexPath := filepath.Join(dir, "probe.apex")
	if err := os.WriteFile(apexPath, []byte(RenderAnonymous(cases)), 0o644); err != nil {
		return Report{}, err
	}
	cmd := exec.CommandContext(ctx, "sf", "apex", "run", "--target-org", opts.TargetOrg, "--file", apexPath)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return Report{}, fmt.Errorf("sf apex run failed: %w\nstdout:\n%s\nstderr:\n%s", err, out.String(), stderr.String())
	}
	results, err := parseResults(out.String() + "\n" + stderr.String())
	if err != nil {
		return Report{}, err
	}
	return Report{TargetOrg: opts.TargetOrg, Results: results}, nil
}

func parseResults(text string) ([]Result, error) {
	for _, line := range strings.Split(text, "\n") {
		idx := strings.Index(line, marker)
		if idx < 0 {
			continue
		}
		raw := strings.TrimSpace(line[idx+len(marker):])
		var results []Result
		if err := json.Unmarshal([]byte(raw), &results); err != nil {
			return nil, fmt.Errorf("decode oracle marker JSON: %w", err)
		}
		for i := range results {
			results[i].RawLogLine = line
		}
		return results, nil
	}
	return nil, fmt.Errorf("oracle marker %q not found in sf output", marker)
}
```

- [ ] **Step 1.4: Add report writer**

Create `/Users/matt/Dev/glade-tools/internal/oracleprobe/write.go`:

```go
package oracleprobe

import (
	"encoding/json"
	"io"
)

func WriteJSON(w io.Writer, report Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}
```

- [ ] **Step 1.5: Add a starter case provider**

Create `/Users/matt/Dev/glade-tools/internal/oracleprobe/stdlib_cases.go`:

```go
package oracleprobe

func StdlibCases() []Case {
	return []Case{
		{ID: "decimal-round-half-up-positive", Area: "Decimal", API: "Decimal.round", Mode: ModeAnonymous, Expression: "Decimal.valueOf('2.5').round()", ValueType: "Integer"},
	}
}
```

- [ ] **Step 1.6: Add focused harness tests**

Create `/Users/matt/Dev/glade-tools/internal/oracleprobe/oracleprobe_test.go`:

```go
package oracleprobe

import (
	"strings"
	"testing"
)

func TestRenderAnonymousIncludesMarkerAndCases(t *testing.T) {
	source := RenderAnonymous([]Case{{
		ID:         "decimal-round-half-up",
		Area:       "Decimal",
		API:        "Decimal.round",
		Mode:       ModeAnonymous,
		Statements: []string{"Decimal __d = Decimal.valueOf('2.5')"},
		Expression: "Decimal.valueOf('2.5').round()",
		ValueType:  "Integer",
	}})
	if !containsAll(source, "GLADE_STDLIB_ORACLE:", "Decimal.valueOf('2.5').round()", "decimal-round-half-up") {
		t.Fatalf("rendered source missing required probe pieces:\n%s", source)
	}
}

func TestParseResultsReadsDebugMarker(t *testing.T) {
	line := `USER_DEBUG [1]|DEBUG|GLADE_STDLIB_ORACLE:[{"id":"x","area":"Decimal","api":"Decimal.round","mode":"anonymous","value":"3","valueType":"Integer"}]`
	results, err := parseResults(line)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].ID != "x" || results[0].Value != "3" {
		t.Fatalf("results = %#v", results)
	}
}

func containsAll(text string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(text, part) {
			return false
		}
	}
	return true
}
```

- [ ] **Step 1.7: Wire the CLI command**

Modify `/Users/matt/Dev/glade-tools/internal/toolcli/compat_command.go`:

```go
case "oracle-stdlib":
	return runCompatOracleStdlib(ctx, args[1:], w)
```

Add a command function in the same file:

```go
func runCompatOracleStdlib(ctx context.Context, args []string, w io.Writer) error {
	targetOrg := ""
	outputPath := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--target-org":
			i++
			if i >= len(args) {
				return errors.New("usage: glade-tools oracle-stdlib --target-org <alias> [--output <path>]")
			}
			targetOrg = args[i]
		case "--output":
			i++
			if i >= len(args) {
				return errors.New("usage: glade-tools oracle-stdlib --target-org <alias> [--output <path>]")
			}
			outputPath = args[i]
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}
	if targetOrg == "" {
		return errors.New("usage: glade-tools oracle-stdlib --target-org <alias> [--output <path>]")
	}
	report, err := oracleprobe.RunAnonymous(ctx, oracleprobe.StdlibCases(), oracleprobe.Options{TargetOrg: targetOrg})
	if err != nil {
		return err
	}
	if outputPath != "" {
		var buf bytes.Buffer
		if err := oracleprobe.WriteJSON(&buf, report); err != nil {
			return err
		}
		return os.WriteFile(outputPath, buf.Bytes(), 0o644)
	}
	return oracleprobe.WriteJSON(w, report)
}
```

Add imports to `compat_command.go`:

```go
"github.com/glade-sh/glade/tools/internal/oracleprobe"
```

The file already imports `bytes`, `context`, `errors`, `fmt`, `io`, and `os`.

- [ ] **Step 1.8: Run harness tests**

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported-tools
go test ./internal/oracleprobe ./internal/toolcli
```

Expected: pass.

### Task 2: Add Oracle Probe Cases

**Files:**
- Modify: `/Users/matt/Dev/glade-tools/internal/oracleprobe/stdlib_cases.go`
- Create generated output: `/Users/matt/Dev/glade-tools/docs/fixtures/oracle/stdlib/core-stdlib-oracle.json`
- Create generated output: `/Users/matt/Dev/glade-tools/docs/fixtures/oracle/stdlib/json-dto-oracle.json`

- [ ] **Step 2.1: Add the first case matrix**

Create `/Users/matt/Dev/glade-tools/internal/oracleprobe/stdlib_cases.go`:

```go
package oracleprobe

func StdlibCases() []Case {
	return []Case{
		{ID: "decimal-round-half-up-positive", Area: "Decimal", API: "Decimal.round", Mode: ModeAnonymous, Expression: "Decimal.valueOf('2.5').round()", ValueType: "Integer"},
		{ID: "decimal-round-half-up-negative", Area: "Decimal", API: "Decimal.round", Mode: ModeAnonymous, Expression: "Decimal.valueOf('-2.5').round()", ValueType: "Integer"},
		{ID: "decimal-round-half-even-down", Area: "Decimal", API: "Decimal.round", Mode: ModeAnonymous, Expression: "Decimal.valueOf('12.5').round(RoundingMode.HALF_EVEN)", ValueType: "Integer"},
		{ID: "decimal-round-half-even-up", Area: "Decimal", API: "Decimal.round", Mode: ModeAnonymous, Expression: "Decimal.valueOf('13.5').round(RoundingMode.HALF_EVEN)", ValueType: "Integer"},
		{ID: "decimal-setscale-large-positive-scale", Area: "Decimal", API: "Decimal.setScale", Mode: ModeAnonymous, Expression: "Decimal.valueOf('1.234567890123456789').setScale(18, RoundingMode.HALF_UP)", ValueType: "Decimal"},
		{ID: "decimal-setscale-negative-scale", Area: "Decimal", API: "Decimal.setScale", Mode: ModeAnonymous, Expression: "Decimal.valueOf('125').setScale(-1, RoundingMode.HALF_UP)", ValueType: "Decimal"},
		{ID: "decimal-setscale-unnecessary-success", Area: "Decimal", API: "Decimal.setScale", Mode: ModeAnonymous, Expression: "Decimal.valueOf('1.20').setScale(2, RoundingMode.UNNECESSARY)", ValueType: "Decimal"},
		{ID: "decimal-setscale-unnecessary-throws", Area: "Decimal", API: "Decimal.setScale", Mode: ModeAnonymous, Expression: "Decimal.valueOf('1.21').setScale(1, RoundingMode.UNNECESSARY)", ExpectThrow: true},

		{ID: "encoding-urlencode-utf8", Area: "EncodingUtil", API: "EncodingUtil.urlEncode", Mode: ModeAnonymous, Expression: "EncodingUtil.urlEncode('A B+Ω', 'UTF-8')", ValueType: "String"},
		{ID: "encoding-urldecode-utf8", Area: "EncodingUtil", API: "EncodingUtil.urlDecode", Mode: ModeAnonymous, Expression: "EncodingUtil.urlDecode('A+B%2B%CE%A9', 'utf8')", ValueType: "String"},
		{ID: "encoding-urlencode-latin1", Area: "EncodingUtil", API: "EncodingUtil.urlEncode", Mode: ModeAnonymous, Expression: "EncodingUtil.urlEncode('café trail', 'ISO-8859-1')", ValueType: "String"},
		{ID: "encoding-urldecode-latin1-invalid-byte", Area: "EncodingUtil", API: "EncodingUtil.urlDecode", Mode: ModeAnonymous, Expression: "EncodingUtil.urlDecode('%CE%A9', 'ISO-8859-1')", ValueType: "String"},
		{ID: "encoding-urlencode-ascii-unrepresentable", Area: "EncodingUtil", API: "EncodingUtil.urlEncode", Mode: ModeAnonymous, Expression: "EncodingUtil.urlEncode('é', 'US-ASCII')", ExpectThrow: true},
		{ID: "encoding-urlencode-utf16", Area: "EncodingUtil", API: "EncodingUtil.urlEncode", Mode: ModeAnonymous, Expression: "EncodingUtil.urlEncode('x', 'UTF-16')", ValueType: "String"},

		{ID: "crypto-digest-md5", Area: "Crypto", API: "Crypto.generateDigest", Mode: ModeAnonymous, Expression: "EncodingUtil.convertToHex(Crypto.generateDigest('MD5', Blob.valueOf('abc')))", ValueType: "String"},
		{ID: "crypto-digest-sha1", Area: "Crypto", API: "Crypto.generateDigest", Mode: ModeAnonymous, Expression: "EncodingUtil.convertToHex(Crypto.generateDigest('SHA1', Blob.valueOf('abc')))", ValueType: "String"},
		{ID: "crypto-digest-sha-1", Area: "Crypto", API: "Crypto.generateDigest", Mode: ModeAnonymous, Expression: "EncodingUtil.convertToHex(Crypto.generateDigest('SHA-1', Blob.valueOf('abc')))", ValueType: "String"},
		{ID: "crypto-digest-sha256", Area: "Crypto", API: "Crypto.generateDigest", Mode: ModeAnonymous, Expression: "EncodingUtil.convertToHex(Crypto.generateDigest('SHA256', Blob.valueOf('abc')))", ValueType: "String"},
		{ID: "crypto-digest-sha3-256", Area: "Crypto", API: "Crypto.generateDigest", Mode: ModeAnonymous, Expression: "EncodingUtil.convertToHex(Crypto.generateDigest('SHA3-256', Blob.valueOf('abc')))", ValueType: "String"},
		{ID: "crypto-digest-bad-name", Area: "Crypto", API: "Crypto.generateDigest", Mode: ModeAnonymous, Expression: "Crypto.generateDigest('SHA-999', Blob.valueOf('abc'))", ExpectThrow: true},

		{ID: "string-split-empty-pattern", Area: "String", API: "String.split", Mode: ModeAnonymous, Expression: "JSON.serialize('abc'.split(''))", ValueType: "String"},
		{ID: "string-split-zero-limit", Area: "String", API: "String.split", Mode: ModeAnonymous, Expression: "JSON.serialize('a,,b,'.split(','))", ValueType: "String"},
		{ID: "string-split-negative-limit", Area: "String", API: "String.split", Mode: ModeAnonymous, Expression: "JSON.serialize('a,,b,'.split(',', -1))", ValueType: "String"},
		{ID: "string-split-positive-limit", Area: "String", API: "String.split", Mode: ModeAnonymous, Expression: "JSON.serialize('a,,b,'.split(',', 2))", ValueType: "String"},
		{ID: "string-split-word-boundary", Area: "String", API: "String.split", Mode: ModeAnonymous, Expression: "JSON.serialize('ab cd'.split('\\\\b', -1))", ValueType: "String"},

		{ID: "pattern-matches-lookahead", Area: "Pattern", API: "Pattern.matches", Mode: ModeAnonymous, Expression: "Pattern.matches('(?!^[0-9]*$)(?!^[a-zA-Z]*$)^([!-~]{8,50})$', 'abc123!!')", ValueType: "Boolean"},
		{ID: "pattern-compile-lookbehind-find", Area: "Pattern", API: "Pattern.compile", Mode: ModeAnonymous, Expression: "Pattern.compile('(?<=a)b').matcher('ab').find()", ValueType: "Boolean"},
		{ID: "pattern-compile-named-group", Area: "Pattern", API: "Pattern.compile", Mode: ModeAnonymous, Expression: "Pattern.compile('(?<word>[A-Z]+)').matcher('ABC').groupCount()", ValueType: "Integer"},
		{ID: "pattern-compile-class-intersection", Area: "Pattern", API: "Pattern.compile", Mode: ModeAnonymous, Expression: "Pattern.compile('[a-z&&[^aeiou]]+').matcher('bcdf').matches()", ValueType: "Boolean"},
		{ID: "matcher-find-zero-width", Area: "Pattern", API: "Matcher.find", Mode: ModeAnonymous, Expression: "Pattern.compile('^|$').matcher('abc').find()", ValueType: "Boolean"},
		{ID: "matcher-group-optional-missing", Area: "Pattern", API: "Matcher.group", Mode: ModeAnonymous, Statements: []string{"Matcher __m = Pattern.compile('([A-Z]+)([0-9]+)?').matcher('ABC')", "__m.find()"}, Expression: "__m.group(2)", ValueType: "String"},
		{ID: "matcher-matches-full-string", Area: "Pattern", API: "Matcher.matches", Mode: ModeAnonymous, Expression: "Pattern.compile('[A-Z]+').matcher('ABC').matches()", ValueType: "Boolean"},

		{ID: "json-untyped-numeric-shapes", Area: "JSON", API: "JSON.deserializeUntyped", Mode: ModeAnonymous, Expression: "JSON.serialize(JSON.deserializeUntyped('{\"whole\":12,\"big\":9223372036854775808,\"ratio\":1.25}'))", ValueType: "String"},
		{ID: "json-untyped-duplicate-keys", Area: "JSON", API: "JSON.deserializeUntyped", Mode: ModeAnonymous, Expression: "JSON.serialize(JSON.deserializeUntyped('{\"Name\":\"First\",\"Name\":\"Second\"}'))", ValueType: "String"},
		{ID: "json-serialize-primitive-map", Area: "JSON", API: "JSON.serialize", Mode: ModeAnonymous, Expression: "JSON.serialize(new Map<String,Object>{'b'=>2,'a'=>1,'n'=>null})", ValueType: "String"},
		{ID: "json-serialize-pretty-map", Area: "JSON", API: "JSON.serializePretty", Mode: ModeAnonymous, Expression: "JSON.serializePretty(new Map<String,Object>{'b'=>2,'a'=>1,'n'=>null})", ValueType: "String"},
		{ID: "json-deserialize-string-key-map", Area: "JSON", API: "JSON.deserialize", Mode: ModeAnonymous, Expression: "JSON.serialize((Map<String,Integer>)JSON.deserialize('{\"a\":1,\"b\":null}', Map<String,Integer>.class))", ValueType: "String"},
		{ID: "json-deserialize-id-key-map", Area: "JSON", API: "JSON.deserialize", Mode: ModeAnonymous, Expression: "JSON.serialize((Map<Id,String>)JSON.deserialize('{\"001B000001DVM9t\":\"Acme\"}', Map<Id,String>.class))", ValueType: "String"},
		{ID: "json-strict-duplicate-fields", Area: "JSON", API: "JSON.deserializeStrict", Mode: ModeAnonymous, Expression: "JSON.deserializeStrict('{\"Name\":\"First\",\"Name\":\"Second\"}', Account.class)", ExpectThrow: true},
		{ID: "json-strict-unknown-sobject-field", Area: "JSON", API: "JSON.deserializeStrict", Mode: ModeAnonymous, Expression: "JSON.deserializeStrict('{\"Name\":\"Acme\",\"NoSuchField__c\":\"x\"}', Account.class)", ExpectThrow: true},
	}
}
```

- [ ] **Step 2.2: Generate the first oracle fixture**

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported-tools
mkdir -p docs/fixtures/oracle/stdlib
go run ./cmd/glade-tools oracle-stdlib \
  --target-org oaer-probe-max \
  --output docs/fixtures/oracle/stdlib/core-stdlib-oracle.json
jq '.results | length' docs/fixtures/oracle/stdlib/core-stdlib-oracle.json
```

Expected: the count equals `len(StdlibCases())`.

- [ ] **Step 2.3: Inspect throws before implementation**

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported-tools
jq -r '.results[] | select(.exceptionType != null) | [.id,.exceptionType,.exceptionMessage] | @tsv' docs/fixtures/oracle/stdlib/core-stdlib-oracle.json
```

Expected: only cases marked `ExpectThrow: true` throw. If an unmarked case throws, update the case expectation and record the behavior in the implementation notes. Do not discard the case.

- [ ] **Step 2.4: Capture deployed DTO JSON behavior**

Create a temporary SFDX source tree:

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported-tools
tmp="$(mktemp -d)"
mkdir -p "$tmp/force-app/main/default/classes"
```

Create `$tmp/force-app/main/default/classes/GladeJSONDTO.cls`:

```apex
public class GladeJSONDTO {
    public String Name;
    public Integer Count;
    public List<String> Tags;
    public transient String Secret;
    public Inner Primary;
    public Map<String, Inner> AddressBook;

    public String Computed {
        get { return Name == null ? null : Name + '-computed'; }
        set;
    }

    public class Inner {
        public String City;
        public Integer Zip;
    }
}
```

Create `$tmp/force-app/main/default/classes/GladeJSONDTO.cls-meta.xml`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata">
    <apiVersion>65.0</apiVersion>
    <status>Active</status>
</ApexClass>
```

Create `$tmp/force-app/main/default/classes/GladeJSONDTOProbe.cls`:

```apex
public class GladeJSONDTOProbe {
    private static Map<String,Object> ok(String id, Object value) {
        return new Map<String,Object>{'id' => id, 'value' => String.valueOf(value), 'valueType' => value == null ? 'null' : 'Object'};
    }

    private static Map<String,Object> err(String id, Exception e) {
        return new Map<String,Object>{'id' => id, 'exceptionType' => e.getTypeName(), 'exceptionMessage' => e.getMessage()};
    }

    public static String runAll() {
        List<Object> rows = new List<Object>();
        try {
            GladeJSONDTO dto = (GladeJSONDTO)JSON.deserialize('{"Name":"Ada","Count":7,"Tags":["north","north"],"Secret":"hide","Primary":{"City":"Delta","Zip":99501},"AddressBook":{"home":{"City":"Cabin","Zip":3}}}', GladeJSONDTO.class);
            rows.add(ok('json-dto-deserialize', JSON.serialize(dto)));
        } catch (Exception e) {
            rows.add(err('json-dto-deserialize', e));
        }
        try {
            GladeJSONDTO dto = (GladeJSONDTO)JSON.deserializeStrict('{"Name":"Ada","Nope":"x"}', GladeJSONDTO.class);
            rows.add(ok('json-dto-strict-unknown', JSON.serialize(dto)));
        } catch (Exception e) {
            rows.add(err('json-dto-strict-unknown', e));
        }
        try {
            GladeJSONDTO dto = new GladeJSONDTO();
            dto.Name = 'Ada';
            dto.Count = 7;
            dto.Secret = 'hide';
            rows.add(ok('json-dto-serialize-transient-property', JSON.serialize(dto)));
        } catch (Exception e) {
            rows.add(err('json-dto-serialize-transient-property', e));
        }
        return JSON.serialize(rows);
    }
}
```

Create `$tmp/force-app/main/default/classes/GladeJSONDTOProbe.cls-meta.xml`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata">
    <apiVersion>65.0</apiVersion>
    <status>Active</status>
</ApexClass>
```

Deploy and run:

```bash
sf project deploy start --target-org oaer-probe-max --source-dir "$tmp/force-app" --json
printf "System.debug('GLADE_STDLIB_ORACLE_DEPLOY:' + GladeJSONDTOProbe.runAll());\n" > "$tmp/json-dto.apex"
sf apex run --target-org oaer-probe-max --file "$tmp/json-dto.apex" > "$tmp/json-dto.log"
payload="$(awk -v marker='GLADE_STDLIB_ORACLE_DEPLOY:' 'index($0, marker) { print substr($0, index($0, marker) + length(marker)); exit }' "$tmp/json-dto.log")"
jq -n --arg targetOrg "oaer-probe-max" --argjson results "$payload" '{targetOrg: $targetOrg, results: $results}' \
  > docs/fixtures/oracle/stdlib/json-dto-oracle.json
jq '.results | length' docs/fixtures/oracle/stdlib/json-dto-oracle.json
```

Expected: `3`.

### Task 3: Replace The Decimal Local Model For Supported Round/Scale

**Files:**
- Modify: `/Users/matt/Dev/glade/internal/vm/value.go`
- Modify: `/Users/matt/Dev/glade/internal/vm/stdlib.go`
- Modify: `/Users/matt/Dev/glade/internal/vm/stdlib_number.go`
- Modify: `/Users/matt/Dev/glade/internal/vm/stdlib_test.go`

- [ ] **Step 3.1: Add failing decimal oracle tests**

In `/Users/matt/Dev/glade/internal/vm/stdlib_test.go`, add:

```go
func TestExecDecimalRoundAndSetScaleOracleEdges(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals(3, Decimal.valueOf('2.5').round());
System.assertEquals(-3, Decimal.valueOf('-2.5').round());
System.assertEquals(12, Decimal.valueOf('12.5').round(RoundingMode.HALF_EVEN));
System.assertEquals(14, Decimal.valueOf('13.5').round(RoundingMode.HALF_EVEN));
System.assertEquals(1.234567890123456789, Decimal.valueOf('1.234567890123456789').setScale(18, RoundingMode.HALF_UP));
System.assertEquals(130, Decimal.valueOf('125').setScale(-1, RoundingMode.HALF_UP));
System.assertEquals(1.20, Decimal.valueOf('1.20').setScale(2, RoundingMode.UNNECESSARY));
try {
	Decimal.valueOf('1.21').setScale(1, RoundingMode.UNNECESSARY);
	System.assert(false, 'expected rounding exception');
} catch (MathException e) {
	System.assert(e.getMessage().length() > 0);
}
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}
```

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported
go test ./internal/vm -run TestExecDecimalRoundAndSetScaleOracleEdges -count=1
```

Expected before implementation: fail on the local scale fence or precision mismatch.

- [ ] **Step 3.2: Preserve exact decimal text**

Change `/Users/matt/Dev/glade/internal/vm/value.go` so decimal values keep exact text whenever constructed from text. Keep `Decimal float64` for existing comparisons, but make decimal operations prefer `Value.Text`.

Add helper functions in `/Users/matt/Dev/glade/internal/vm/stdlib.go`:

```go
func decimalText(value Value) string {
	if value.Kind == ValueDecimal && strings.TrimSpace(value.Text) != "" {
		return value.Text
	}
	return strconv.FormatFloat(value.Decimal, 'f', -1, 64)
}

func decimalFromRat(r *big.Rat, scale int64) Value {
	text := ratToScaleString(r, scale)
	floatValue, _ := strconv.ParseFloat(text, 64)
	out := Decimal(floatValue)
	out.Text = text
	return out
}
```

Add `ratToScaleString` beside `roundLocalDecimalStringToScale`. It must write fixed decimal notation for positive, zero, and negative scales. It must not use scientific notation.

- [ ] **Step 3.3: Remove the scale fence for `round` and `setScale`**

Replace `roundDecimalToScale(callee string, value float64, scaleValue int64, mode string) (float64, error)` with:

```go
func roundDecimalValueToScale(callee string, value Value, scaleValue int64, mode string) (Value, error) {
	rat := new(big.Rat)
	if _, ok := rat.SetString(decimalText(value)); !ok {
		return Null, fmt.Errorf("%s value cannot be represented by local decimal model", callee)
	}
	absScale := scaleValue
	if absScale < 0 {
		absScale = -absScale
	}
	factor := new(big.Int).Exp(big.NewInt(10), big.NewInt(absScale), nil)
	factorRat := new(big.Rat).SetInt(factor)
	scaled := new(big.Rat)
	if scaleValue >= 0 {
		scaled.Mul(rat, factorRat)
	} else {
		scaled.Quo(rat, factorRat)
	}
	rounded, err := roundScaledRat(callee, scaled, mode)
	if err != nil {
		return Null, err
	}
	resultRat := new(big.Rat)
	if scaleValue >= 0 {
		resultRat.SetFrac(rounded, factor)
	} else {
		resultRat.Mul(new(big.Rat).SetInt(rounded), factorRat)
	}
	return decimalFromRat(resultRat, scaleValue), nil
}
```

Update `callDecimalMember` in `/Users/matt/Dev/glade/internal/vm/stdlib_number.go`:

- `setScale` returns the `Value` from `roundDecimalValueToScale`.
- `round` calls `roundDecimalValueToScale(..., 0, mode)` and then converts the exact text to `Integer`.

- [ ] **Step 3.4: Match Salesforce exception classes**

Run the oracle fixture and inspect the `decimal-setscale-unnecessary-throws` row. If Salesforce throws `System.MathException`, Glade must throw the same Apex exception type, not a generic Go error.

Add or reuse:

```go
return Null, newExceptionError("MathException", "Rounding necessary")
```

Use the exact oracle message when the fixture provides it.

- [ ] **Step 3.5: Run decimal tests**

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported
go test ./internal/vm -run 'TestExecDecimal|TestExecNumeric|TestDecimal' -count=1
```

Expected: pass.

### Task 4: Finish EncodingUtil And Crypto.generateDigest

**Files:**
- Modify: `/Users/matt/Dev/glade/internal/vm/stdlib.go`
- Modify: `/Users/matt/Dev/glade/internal/vm/stdlib_test.go`
- Modify: `/Users/matt/Dev/glade/go.mod`
- Modify: `/Users/matt/Dev/glade/go.sum`

- [ ] **Step 4.1: Add failing oracle tests**

In `/Users/matt/Dev/glade/internal/vm/stdlib_test.go`, add:

```go
func TestExecEncodingAndDigestOracleEdges(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals('A+B%2B%CE%A9', EncodingUtil.urlEncode('A B+Ω', 'UTF-8'));
System.assertEquals('A B+Ω', EncodingUtil.urlDecode('A+B%2B%CE%A9', 'utf8'));
System.assertEquals('caf%E9+trail', EncodingUtil.urlEncode('café trail', 'ISO-8859-1'));
System.assertEquals('900150983cd24fb0d6963f7d28e17f72', EncodingUtil.convertToHex(Crypto.generateDigest('MD5', Blob.valueOf('abc'))));
System.assertEquals('a9993e364706816aba3e25717850c26c9cd0d89d', EncodingUtil.convertToHex(Crypto.generateDigest('SHA1', Blob.valueOf('abc'))));
System.assertEquals(EncodingUtil.convertToHex(Crypto.generateDigest('SHA1', Blob.valueOf('abc'))), EncodingUtil.convertToHex(Crypto.generateDigest('SHA-1', Blob.valueOf('abc'))));
System.assertEquals(EncodingUtil.convertToHex(Crypto.generateDigest('SHA256', Blob.valueOf('abc'))), EncodingUtil.convertToHex(Crypto.generateDigest('SHA-256', Blob.valueOf('abc'))));
try {
	Crypto.generateDigest('SHA-999', Blob.valueOf('abc'));
	System.assert(false, 'expected security exception');
} catch (SecurityException e) {
	System.assert(e.getMessage().length() > 0);
}
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}
```

Run:

```bash
go test ./internal/vm -run TestExecEncodingAndDigestOracleEdges -count=1
```

Expected before implementation: pass or fail only where oracle behavior differs from the current local model. Differences become implementation work.

- [ ] **Step 4.2: Add charset support through a real charset registry**

If oracle rows show `UTF-16` or additional aliases are supported, add:

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported
go get golang.org/x/text/encoding@latest golang.org/x/text/encoding/ianaindex@latest golang.org/x/text/transform@latest
```

Update URL encode/decode helpers in `/Users/matt/Dev/glade/internal/vm/stdlib.go`:

- Normalize names through `ianaindex.MIB.Encoding`.
- Preserve existing UTF-8, ASCII, and ISO-8859-1 aliases.
- Match oracle behavior for unrepresentable characters.
- Match oracle behavior for malformed percent escapes.
- Throw the oracle exception type and message for unsupported charset names.

- [ ] **Step 4.3: Confirm digest aliases**

Update the digest algorithm normalizer so every oracle-supported alias maps to the correct hash. Keep unsupported names as `System.SecurityException`.

The minimum supported aliases must include:

```text
MD5
SHA1
SHA-1
SHA256
SHA-256
SHA384
SHA-384
SHA512
SHA-512
SHA3-256
SHA3-384
SHA3-512
```

- [ ] **Step 4.4: Run tests**

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported
go test ./internal/vm -run 'TestExecEncoding|TestExecCrypto|TestExecEncodingAndDigestOracleEdges' -count=1
```

Expected: pass.

### Task 5: Replace Regex/String Split Fences With Java-Compatible Matching

**Files:**
- Modify: `/Users/matt/Dev/glade/go.mod`
- Modify: `/Users/matt/Dev/glade/go.sum`
- Create: `/Users/matt/Dev/glade/internal/vm/apex_regex.go`
- Modify: `/Users/matt/Dev/glade/internal/vm/stdlib.go`
- Modify: `/Users/matt/Dev/glade/internal/vm/stdlib_regex.go`
- Modify: `/Users/matt/Dev/glade/internal/vm/regex_test.go`
- Modify: `/Users/matt/Dev/glade/internal/vm/stdlib_test.go`

- [ ] **Step 5.1: Add failing Java regex tests**

In `/Users/matt/Dev/glade/internal/vm/regex_test.go`, add:

```go
func TestExecRegexOracleJavaFeatures(t *testing.T) {
	program, err := CompileAnonymous(`
System.assert(Pattern.compile('(?<=a)b').matcher('ab').find());
System.assert(Pattern.compile('[a-z&&[^aeiou]]+').matcher('bcdf').matches());
System.assert(Pattern.compile('(?<word>[A-Z]+)').matcher('ABC').matches());
System.assert(Pattern.matches('(?!^[0-9]*$)(?!^[a-zA-Z]*$)^([!-~]{8,50})$', 'abc123!!'));
System.assertEquals(JSON.serialize(new List<String>{'a','','b',''}), JSON.serialize('a,,b,'.split(',', -1)));
System.assertEquals(JSON.serialize(new List<String>{'a','','b'}), JSON.serialize('a,,b,'.split(',')));
System.assertEquals(JSON.serialize(new List<String>{'','a','b','c',''}), JSON.serialize('abc'.split('', -1)));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}
```

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported
go test ./internal/vm -run TestExecRegexOracleJavaFeatures -count=1
```

Expected before implementation: fail on current Java-only fences.

- [ ] **Step 5.2: Add a regex adapter**

Add a pure-Go regex engine behind `/Users/matt/Dev/glade/internal/vm/apex_regex.go`. Start with:

```bash
go get github.com/dlclark/regexp2@latest
```

The adapter must expose this local interface:

```go
type apexRegex struct {
	source string
	flags  int64
}

type apexRegexMatch struct {
	start  int
	end    int
	groups []apexRegexGroup
}

type apexRegexGroup struct {
	matched bool
	start   int
	end     int
	text    string
}

func compileApexRegex(source string, flags int64) (*apexRegex, error)
func (r *apexRegex) find(input string, startRune int, endRune int) (*apexRegexMatch, error)
func (r *apexRegex) matches(input string, startRune int, endRune int) (*apexRegexMatch, error)
func (r *apexRegex) split(input string, limit int64) ([]string, error)
```

The adapter owns Java-to-engine normalization:

- `Pattern.CASE_INSENSITIVE`
- `Pattern.MULTILINE`
- `Pattern.DOTALL`
- `Pattern.LITERAL`
- `Pattern.UNICODE_CASE`
- empty regex matches
- zero-width split advancement
- numeric backreferences
- named capture syntax
- lookahead
- fixed-width lookbehind
- possessive quantifiers
- character-class intersections

If the chosen engine cannot express a Salesforce-supported case, add code in the adapter or fork the dependency. Do not leave the row fenced.

- [ ] **Step 5.3: Route Pattern and Matcher through the adapter**

Update `/Users/matt/Dev/glade/internal/vm/stdlib.go` and `/Users/matt/Dev/glade/internal/vm/stdlib_regex.go`:

- `Pattern.compile` stores the original source, flags, and adapter-validated source.
- `Pattern.matches` calls `compileApexRegex(...).matches(...)`.
- `Pattern.matcher` stores the compiled adapter metadata on the matcher.
- `Matcher.find`, `Matcher.matches`, and `Matcher.group` use adapter match/group state.
- `String.split` and `Pattern.split` use `apexRegex.split`.
- Remove unsupported errors for Java regex features that pass oracle probes.
- Keep `PatternSyntaxException` for syntax that Salesforce rejects.

- [ ] **Step 5.4: Run regex tests**

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported
go test ./internal/vm -run 'TestExecPattern|TestMatcher|TestRegex|TestExecStringSplit' -count=1
```

Expected: pass. No target row can still contain an unsupported diagnostic for Java regex features proved by the oracle fixture.

### Task 6: Finish JSON Untyped, Serialize, And Pretty Output

**Files:**
- Modify: `/Users/matt/Dev/glade/internal/vm/json_runtime.go`
- Modify: `/Users/matt/Dev/glade/internal/vm/json_parser.go`
- Modify: `/Users/matt/Dev/glade/internal/vm/json_generator.go`
- Modify: `/Users/matt/Dev/glade/internal/vm/json_test.go`

- [ ] **Step 6.1: Add failing JSON core oracle tests**

In `/Users/matt/Dev/glade/internal/vm/json_test.go`, add:

```go
func TestExecJSONCoreOracleEdges(t *testing.T) {
	program, err := CompileAnonymous(`
Object raw = JSON.deserializeUntyped('{"whole":12,"big":9223372036854775808,"ratio":1.25,"items":[1,"two",false,{"inner":null}]}');
String rawText = JSON.serialize(raw);
System.assert(rawText.contains('"whole"'));
System.assert(rawText.contains('"ratio"'));
Object dup = JSON.deserializeUntyped('{"Name":"First","Name":"Second"}');
System.assertEquals('Second', ((Map<String,Object>)dup).get('Name'));
Map<String,Object> values = new Map<String,Object>{'b'=>2,'a'=>1,'n'=>null};
String compact = JSON.serialize(values);
String pretty = JSON.serializePretty(values);
System.assert(compact.contains('"a"'));
System.assert(pretty.contains('\n'));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}
```

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported
go test ./internal/vm -run TestExecJSONCoreOracleEdges -count=1
```

Expected before implementation: fail only where oracle output shows a local mismatch.

- [ ] **Step 6.2: Match untyped number and duplicate-key behavior**

Update `/Users/matt/Dev/glade/internal/vm/json_runtime.go`:

- Keep `json.Decoder.UseNumber`.
- Map JSON integers within Apex `Integer` range to `ValueInt`.
- Map larger integers and decimal numbers according to oracle results.
- Preserve duplicate-key behavior from the oracle: non-strict deserialization keeps the last field value.
- Keep strict duplicate detection in `deserializeStrict`.

- [ ] **Step 6.3: Match compact and pretty generator output**

Update `/Users/matt/Dev/glade/internal/vm/json_runtime.go` and `/Users/matt/Dev/glade/internal/vm/json_generator.go`:

- Preserve Salesforce field ordering observed by probes.
- Preserve null suppression behavior for `suppressApexObjectNulls`.
- Match escaping for `<`, `>`, `&`, Unicode, slash, backslash, newline, carriage return, tab, and quote.
- Match pretty indentation and field separator spacing from oracle output.

- [ ] **Step 6.4: Run JSON core tests**

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported
go test ./internal/vm -run 'TestExecJSON(Core|Serialize|Generator|Parser|DeserializeUntyped)' -count=1
```

Expected: pass.

### Task 7: Finish JSON Typed, Strict, DTO, And SObject Coercion

**Files:**
- Modify: `/Users/matt/Dev/glade/internal/vm/json_runtime.go`
- Modify: `/Users/matt/Dev/glade/internal/vm/json_test.go`
- Modify: `/Users/matt/Dev/glade/internal/typesys/system_stub_symbols_generated.go` only if a missing type shape blocks a supported JSON target.

- [ ] **Step 7.1: Add typed JSON oracle tests**

In `/Users/matt/Dev/glade/internal/vm/json_test.go`, add:

```go
func TestExecJSONTypedOracleEdges(t *testing.T) {
	program, err := CompileAnonymous(`
Integer n = JSON.deserialize('7', Integer.class);
System.assertEquals(7, n);
Map<String,Integer> counts = JSON.deserialize('{"a":1,"b":null}', Map<String,Integer>.class);
System.assertEquals(1, counts.get('a'));
System.assertEquals(null, counts.get('b'));
Map<Id,String> byId = JSON.deserialize('{"001B000001DVM9t":"Acme"}', Map<Id,String>.class);
System.assertEquals('Acme', byId.get((Id)'001B000001DVM9t'));
Account account = JSON.deserialize('{"Name":"Acme","AnnualRevenue":12.5,"ParentId":"001B000001DVM9t"}', Account.class);
System.assertEquals('Acme', account.Name);
try {
	JSON.deserializeStrict('{"Name":"First","Name":"Second"}', Account.class);
	System.assert(false, 'expected duplicate strict field');
} catch (JSONException e) {
	System.assert(e.getMessage().length() > 0);
}
try {
	JSON.deserializeStrict('{"Name":"Acme","NoSuchField__c":"x"}', Account.class);
	System.assert(false, 'expected unknown strict field');
} catch (JSONException e) {
	System.assert(e.getMessage().length() > 0);
}
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}
```

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported
go test ./internal/vm -run TestExecJSONTypedOracleEdges -count=1
```

Expected before implementation: fail on any still-fenced target coercion.

- [ ] **Step 7.2: Remove unsupported map-key fences for supported Apex key types**

Update `typedValueFromJSON` and related map coercion helpers in `/Users/matt/Dev/glade/internal/vm/json_runtime.go`.

The supported key target set must include every key type that the oracle accepts among:

```text
String
Id
Integer
Long
Decimal
Boolean
Date
Datetime
Time
Enum values already known to the VM
```

For each key type:

- Parse the JSON object key string through the same scalar parser used for field values.
- Store `MapKeys` with the typed key value.
- Keep deterministic `MapOrder`.
- Throw `JSONException` with the oracle message when a key string cannot coerce.

- [ ] **Step 7.3: Match strict known-field behavior**

Update strict DTO/SObject coercion:

- Unknown DTO field: throw `JSONException`.
- Unknown SObject field: throw `JSONException`.
- Duplicate field: throw `JSONException`.
- Known relationship object: coerce through relationship metadata.
- Known child relationship records: coerce into local relationship list shape.
- Static and transient fields: match oracle behavior.
- Property setters: call setters when the local VM has one.
- Private fields: match oracle behavior for local Apex DTOs.

- [ ] **Step 7.4: Run JSON typed tests**

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported
go test ./internal/vm -run 'TestExecJSONDeserialize|TestTypedValueFromJSON|TestExecJSONTypedOracleEdges' -count=1
```

Expected: pass.

### Task 8: Convert Oracle Output Into Checked Fixtures

**Files:**
- Create: `/Users/matt/Dev/glade-tools/docs/fixtures/oracle/stdlib/core-stdlib-oracle.json`
- Create: `/Users/matt/Dev/glade-tools/docs/fixtures/core-stdlib-supported-closeout.json`

- [ ] **Step 8.1: Keep raw oracle evidence**

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported-tools
go run ./cmd/glade-tools oracle-stdlib \
  --target-org oaer-probe-max \
  --output docs/fixtures/oracle/stdlib/core-stdlib-oracle.json
jq -e '.targetOrg == "oaer-probe-max" and (.results | length > 0)' docs/fixtures/oracle/stdlib/core-stdlib-oracle.json
```

Expected:

```text
true
```

- [ ] **Step 8.2: Add a compat fixture for the local closeout**

Create `/Users/matt/Dev/glade-tools/docs/fixtures/core-stdlib-supported-closeout.json` using the existing fixture schema. The Apex body must cover all rows in this plan:

```apex
System.assertEquals(3, Decimal.valueOf('2.5').round());
System.assertEquals(130, Decimal.valueOf('125').setScale(-1, RoundingMode.HALF_UP));
System.assertEquals('A+B%2B%CE%A9', EncodingUtil.urlEncode('A B+Ω', 'UTF-8'));
System.assertEquals('900150983cd24fb0d6963f7d28e17f72', EncodingUtil.convertToHex(Crypto.generateDigest('MD5', Blob.valueOf('abc'))));
System.assert(Pattern.compile('(?<=a)b').matcher('ab').find());
System.assert(Pattern.compile('[a-z&&[^aeiou]]+').matcher('bcdf').matches());
System.assertEquals(JSON.serialize(new List<String>{'a','','b',''}), JSON.serialize('a,,b,'.split(',', -1)));
Object raw = JSON.deserializeUntyped('{"whole":12,"ratio":1.25}');
System.assert(JSON.serialize(raw).contains('"whole"'));
Map<Id,String> byId = JSON.deserialize('{"001B000001DVM9t":"Acme"}', Map<Id,String>.class);
System.assertEquals('Acme', byId.get((Id)'001B000001DVM9t'));
System.assert(JSON.serializePretty(new Map<String,Object>{'a'=>1}).contains('\n'));
```

- [ ] **Step 8.3: Validate and run the fixture**

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported-tools
go run ./cmd/glade-tools validate docs/fixtures/core-stdlib-supported-closeout.json
go run ./cmd/glade-tools run docs/fixtures/core-stdlib-supported-closeout.json
```

Expected: both pass.

### Task 9: Promote Catalog Rows And Regenerate Docs

**Files:**
- Modify: `/Users/matt/Dev/glade-tools/internal/capability/stdlib.go`
- Modify generated docs in `/Users/matt/Dev/glade/docs`
- Modify public support docs in `/Users/matt/Dev/glade/site/docs-src/guide/support-map.md`

- [ ] **Step 9.1: Promote rows in the catalog**

In `/Users/matt/Dev/glade-tools/internal/capability/stdlib.go`, change the 16 target rows to `StatusSupported`.

Use notes in this shape:

```go
{Area: "Decimal", API: "Decimal.round", Status: StatusSupported, Notes: "Matches Salesforce oracle fixtures for supported Decimal rounding modes, ties, signs, and exact decimal text."},
```

Every promoted note must mention oracle-backed support. No note may keep these phrases:

```text
partial
subset
fenced
not modeled
unsupported map key targets
Java-only regex features remain fenced
local scale fence
```

- [ ] **Step 9.2: Add a catalog guard test**

Add to `/Users/matt/Dev/glade-tools/internal/capability/capability_test.go`:

```go
func TestStdlibTargetRowsAreSupported(t *testing.T) {
	want := map[string]bool{
		"String:String.split": true,
		"Decimal:Decimal.round": true,
		"Decimal:Decimal.setScale": true,
		"JSON:JSON.deserialize": true,
		"JSON:JSON.deserializeStrict": true,
		"JSON:JSON.deserializeUntyped": true,
		"JSON:JSON.serialize": true,
		"JSON:JSON.serializePretty": true,
		"Pattern:Pattern.compile": true,
		"Pattern:Pattern.matches": true,
		"Pattern:Matcher.find": true,
		"Pattern:Matcher.group": true,
		"Pattern:Matcher.matches": true,
		"EncodingUtil:EncodingUtil.urlDecode": true,
		"EncodingUtil:EncodingUtil.urlEncode": true,
		"Crypto:Crypto.generateDigest": true,
	}
	for _, entry := range StdlibMatrix() {
		key := entry.Area + ":" + entry.API
		if !want[key] {
			continue
		}
		if entry.Status != StatusSupported {
			t.Fatalf("%s status = %s, want supported", key, entry.Status)
		}
		delete(want, key)
	}
	if len(want) != 0 {
		t.Fatalf("missing target rows: %#v", want)
	}
}
```

- [ ] **Step 9.3: Regenerate checked docs**

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported-tools
go run ./cmd/glade-tools stdlib --output /Users/matt/Dev/glade/.worktrees/stdlib-supported/docs/STDLIB_COVERAGE.md
go run ./cmd/glade-tools dashboard --output /Users/matt/Dev/glade/.worktrees/stdlib-supported/docs/COMPATIBILITY_DASHBOARD.md
go run ./cmd/glade-tools gaps --output /Users/matt/Dev/glade/.worktrees/stdlib-supported/docs/KNOWN_GAPS.md
```

Expected: generated docs contain the target rows as `supported`.

- [ ] **Step 9.4: Update public support-map wording**

Edit `/Users/matt/Dev/glade/.worktrees/stdlib-supported/site/docs-src/guide/support-map.md`.

The standard-library section must say:

```markdown
Core String, Decimal, JSON, Pattern/Matcher, EncodingUtil, and Crypto digest behavior is supported locally for the checked Apex standard-library rows. The checked coverage table is generated from the first-party compat catalog and links support claims to oracle-backed fixtures.
```

Do not claim broader hosted crypto, certificate, key-store, or encryption service support from this work.

### Task 10: Full Verification

**Files:**
- No new files.

- [ ] **Step 10.1: Product focused tests**

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported
go test ./internal/vm -run 'TestExecDecimal|TestExecEncoding|TestExecCrypto|TestExecPattern|TestMatcher|TestRegex|TestExecStringSplit|TestExecJSON|TestTypedValueFromJSON' -count=1
```

Expected: pass.

- [ ] **Step 10.2: Product full tests**

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported
go test ./...
```

Expected: pass.

- [ ] **Step 10.3: Tools focused and full tests**

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported-tools
go test ./internal/oracleprobe ./internal/toolcli ./internal/capability
go test ./...
```

Expected: pass.

- [ ] **Step 10.4: Fixture validation**

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported-tools
go run ./cmd/glade-tools validate docs/fixtures/core-stdlib-supported-closeout.json
go run ./cmd/glade-tools run docs/fixtures/core-stdlib-supported-closeout.json
jq -e '.results | length > 0' docs/fixtures/oracle/stdlib/core-stdlib-oracle.json
```

Expected: pass and `true`.

- [ ] **Step 10.5: Site checks**

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported
npm --prefix site test
npm --prefix site run build
```

Expected: pass.

- [ ] **Step 10.6: Final support-row audit**

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported-tools
go run ./cmd/glade-tools stdlib --json | jq -r '
  .[] |
  select((.area == "String" and .api == "String.split") or
    (.area == "Decimal" and (.api == "Decimal.round" or .api == "Decimal.setScale")) or
    (.area == "JSON") or
    (.area == "Pattern") or
    (.area == "EncodingUtil") or
    (.area == "Crypto" and .api == "Crypto.generateDigest")) |
  [.area, .api, .status, .notes] | @tsv
'
```

Expected: all 16 target rows print `supported`.

### Task 11: Merge And Cleanup

**Files:**
- No new files.

- [ ] **Step 11.1: Commit tools work**

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported-tools
git status --short
git add internal/oracleprobe internal/toolcli internal/capability docs/fixtures
git commit -m "test: add stdlib oracle parity evidence"
```

Expected: commit succeeds.

- [ ] **Step 11.2: Commit product work**

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported
git status --short
git add go.mod go.sum internal/vm docs/STDLIB_COVERAGE.md docs/COMPATIBILITY_DASHBOARD.md docs/KNOWN_GAPS.md site/docs-src/guide/support-map.md
git commit -m "feat: promote core stdlib partials to supported"
```

Expected: commit succeeds.

- [ ] **Step 11.3: Merge to local main after review**

Run only after review passes:

```bash
cd /Users/matt/Dev/glade-tools
git checkout main
git merge --no-ff codex/stdlib-supported-tools

cd /Users/matt/Dev/glade
git checkout main
git merge --no-ff codex/stdlib-supported
```

Expected: both merges succeed.

- [ ] **Step 11.4: Clean worktrees**

Run only after both merges land:

```bash
cd /Users/matt/Dev/glade
git worktree remove .worktrees/stdlib-supported
git worktree remove .worktrees/stdlib-supported-tools
```

Expected: worktrees removed.

## Risk Notes

- Regex is the hardest log. Go `regexp` cannot provide Java regex parity. The adapter must own the difference, or the dependency must be forked.
- Decimal support requires exact text math. `float64` can stay as a compatibility cache, but it cannot drive supported `round` and `setScale`.
- JSON has the broadest state space. Do not promote typed JSON until SObject, DTO, map key, strict duplicate, unknown field, and scalar coercion probes pass.
- EncodingUtil behavior hinges on Salesforce charset replacement/error semantics. Let the oracle settle that before choosing strict errors or replacement characters.
- `Crypto.generateDigest` is bounded. Do not use this work to claim support for key stores, certificates, random generation, encryption services, or broader Crypto surfaces.

## Final Done State

The work is complete when:

- The oracle fixture exists and records scratch-org behavior.
- The local closeout fixture validates and runs.
- Product and tools `go test ./...` pass.
- Site tests and build pass.
- The 16 target rows are `supported` in `glade-tools stdlib --json`.
- Generated docs and public support map describe the new support without partial/fence wording.
