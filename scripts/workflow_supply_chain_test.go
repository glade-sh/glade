package scripts

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var (
	workflowUsesPattern         = regexp.MustCompile(`^\s*(?:-\s*)?uses:\s*([^\s#]+)`)
	fullActionRefPattern        = regexp.MustCompile(`^[^@\s]+@[0-9a-f]{40}$`)
	actionVersionCommentPattern = regexp.MustCompile(`#\s+v[0-9]`)
	goToolCommandPattern        = regexp.MustCompile(`\bgo\s+(?:run|install)\s+([^\s"']+)`)
	exactToolSemverPattern      = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`)
	mutableRefPattern           = regexp.MustCompile(`@(latest|main|master|HEAD)\b`)
)

func TestWorkflowExecutableReferencesAreImmutable(t *testing.T) {
	workflowDir := filepath.Join("..", ".github", "workflows")
	paths, err := filepath.Glob(filepath.Join(workflowDir, "*.y*ml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatal("no workflow files found")
	}

	querySuiteUses := 0
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for lineNumber, line := range strings.Split(string(data), "\n") {
			problem, querySuite := workflowExecutableReferenceProblem(filepath.Base(path), line)
			if problem != "" {
				t.Errorf("%s:%d: %s", path, lineNumber+1, problem)
			}
			if querySuite {
				querySuiteUses++
			}
		}
	}
	if querySuiteUses != 2 {
		t.Fatalf("CodeQL query-suite uses = %d, want exactly 2 event-specific security-extended configs", querySuiteUses)
	}
}

func TestWorkflowExecutableReferencePolicyRejectsMutableFixtures(t *testing.T) {
	cases := []struct {
		name string
		file string
		line string
	}{
		{name: "moving action tag", file: "ci.yml", line: "      - uses: actions/checkout@v6"},
		{name: "short action sha", file: "ci.yml", line: "      - uses: actions/checkout@df4cb1c"},
		{name: "mutable Go tool", file: "security.yml", line: "        run: go run golang.org/x/vuln/cmd/govulncheck@latest ./..."},
		{name: "branch Go tool", file: "security.yml", line: "        run: go run example.com/tool@main ./..."},
		{name: "unversioned remote Go tool", file: "release.yml", line: "        run: go install example.com/tool/cmd/tool"},
		{name: "query suite outside CodeQL workflow", file: "ci.yml", line: "              - uses: security-extended"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			problem, _ := workflowExecutableReferenceProblem(tc.file, tc.line)
			if problem == "" {
				t.Fatal("mutable executable reference accepted")
			}
		})
	}
}

func workflowExecutableReferenceProblem(file, line string) (string, bool) {
	if match := workflowUsesPattern.FindStringSubmatch(line); match != nil {
		ref := match[1]
		if ref == "security-extended" {
			if file != "security.yml" {
				return "security-extended is only valid in the CodeQL query-suite config", false
			}
			return "", true
		}
		if strings.HasPrefix(ref, "./") {
			return "", false
		}
		if !fullActionRefPattern.MatchString(ref) {
			return fmt.Sprintf("workflow action is not pinned to a full 40-hex commit: %s", ref), false
		}
		if !actionVersionCommentPattern.MatchString(line) {
			return fmt.Sprintf("workflow action lacks a version comment: %s", ref), false
		}
	}

	code := line
	if comment := strings.Index(code, "#"); comment >= 0 {
		code = code[:comment]
	}
	if mutable := mutableRefPattern.FindString(code); mutable != "" {
		return fmt.Sprintf("mutable executable reference: %s", mutable), false
	}
	for _, match := range goToolCommandPattern.FindAllStringSubmatch(code, -1) {
		target := match[1]
		if strings.HasPrefix(target, "./") || strings.HasPrefix(target, "../") {
			continue
		}
		at := strings.LastIndex(target, "@")
		if at < 0 {
			return fmt.Sprintf("remote Go tool lacks an exact version: %s", target), false
		}
		if version := target[at+1:]; !exactToolSemverPattern.MatchString(version) {
			return fmt.Sprintf("remote Go tool lacks an exact stable semantic version: %s", target), false
		}
	}
	return "", false
}

func TestVSCodeWorkflowUsesReadOnlyCheckout(t *testing.T) {
	path := filepath.Join("..", ".github", "workflows", "vscode-glade.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	workflow := string(data)
	header, _, found := strings.Cut(workflow, "\njobs:\n")
	if !found {
		t.Fatal("VS Code workflow lacks jobs boundary")
	}
	if !strings.Contains(header, "\npermissions:\n  contents: read\n") {
		t.Fatal("VS Code workflow lacks top-level contents: read permission")
	}
	if strings.Contains(header, "contents: write") || strings.Contains(header, "id-token: write") {
		t.Fatal("VS Code workflow header grants write authority")
	}
	checkout := "uses: actions/checkout@34e114876b0b11c390a56381ad16ebd13914f8d5 # v4.3.1\n        with:\n          persist-credentials: false"
	if !strings.Contains(workflow, checkout) {
		t.Fatal("VS Code checkout must disable credential persistence")
	}
}
