package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestConformancePairingFixtureDiscoveryReview12(t *testing.T) {
	for _, tc := range []struct {
		name, support, client string
		reject                bool
	}{
		{"all_components_split", ``, `import fp "path/filepath"
func consume() string { return fp.Join("..", "sup"+"port", "test"+"data", "fam"+"ily", "ca"+"se.tsv") }`, false},
		{"aliased_join", ``, `import fp "path/filepath"
func consume() string { return fp.Join("..", "support", "testdata", "family", "case.tsv") }`, false},
		{"aliased_path_join", ``, `import p "path"
func consume() string { return p.Join("..", "support", "testdata", "family", "case.tsv") }`, false},
		{"bound_join", ``, `import fp "path/filepath"
func consume() string { root := "../support"; dir := "testdata"; return fp.Join(root, dir, "family", "case.tsv") }`, false},
		{"cross_file_concat", ``, `func consume() string { return base + fixture }`, true},
		{"cross_file_join", ``, `import fp "path/filepath"
func consume() string { return fp.Join(base, fixture) }`, true},
		{"cross_file_format", ``, `import "fmt"
func consume() string { return fmt.Sprintf("%s%s", base, fixture) }`, true},
		{"cross_file_concat_reverse", ``, `func consume() string { prefix := base; return prefix + fixture }`, true},
		{"cross_nested_group_call", ``, `func consume() string { return base + fixture }`, true},
		{"cross_nested_group_nested", ``, `func consume() string { return base + fixture }`, true},
		{"cross_group_var", ``, `func consume() string { return base + fixture }`, true},
		{"cross_group_const", ``, `func consume() string { return base + fixture }`, true},
		{"cross_group_var_typed", ``, `func consume() string { return base + fixture }`, true},
		{"cross_group_const_typed", ``, `func consume() string { return base + fixture }`, true},
		{"split_join", ``, `import fp "path/filepath"
func consume() string { return fp.Join("..", "support", "test"+"data", "family", "case.tsv") }`, false},
		{"nested_composed", ``, `import fp "path/filepath"
func consume() string { return fp.Join(fp.Join("..", "support", "test"+"data"), "family", "case.tsv") }`, true},
		{"nested_bound", ``, `import fp "path/filepath"
func consume() string { d := "test"+"data"; return fp.Join(fp.Join("..", "support", d), "family", "case.tsv") }`, true},
		{"nested_outer_split", ``, `import fp "path/filepath"
func consume() string { return fp.Join(fp.Join("..", "support"), "test"+"data", "family", "case.tsv") }`, true},
		{"nested_outer_bound", ``, `import fp "path/filepath"
func consume() string { d := "test"+"data"; return fp.Join(fp.Join("..", "support"), d, "family", "case.tsv") }`, true},
		{"variadic_composed", ``, `import fp "path/filepath"
func consume() string { p := []string{"..", "support", "test"+"data", "family", "case.tsv"}; return fp.Join(p...) }`, true},
		{"variadic_bound", ``, `import fp "path/filepath"
func consume() string { d := "test"+"data"; p := []string{"..", "support", d, "family", "case.tsv"}; return fp.Join(p...) }`, true},
		{"variable_prefix", ``, `func consume() string { root := "../support/"; return root + "testdata/family/case.tsv" }`, false},
		{"variable_prefix_split", ``, `func consume() string { root := "../"; return root + "support/testdata/family/case.tsv" }`, false},
		{"strings_join", ``, `import "strings"
func consume() string { return strings.Join([]string{"..", "support", "testdata", "family", "case.tsv"}, "/") }`, false},
		{"strings_join_alias", ``, `import s "strings"
func consume() string { return s.Join([]string{"../support", "testdata/family", "case.tsv"}, "/") }`, false},
		{"strings_join_slice", ``, `import "strings"
func consume() string { parts := []string{"..", "support", "testdata", "family", "case.tsv"}; return strings.Join(parts, "/") }`, false},
		{"composed_format", ``, `import "fmt"
func consume() string { return fmt.Sprintf("../%s/%s", "support/testdata", "family/case.tsv") }`, true},
		{"composed_format_alias", ``, `import f "fmt"
func consume() string { return f.Sprintf("../%s/%s", "support/testdata", "family/case.tsv") }`, true},
		{"split_format_arguments", ``, `import "fmt"
func consume() string { return fmt.Sprintf("../support/%s%s/%s", "test", "data", "family/case.tsv") }`, true},
		{"split_format_prefix", ``, `import "fmt"
func consume() string { return fmt.Sprintf("../support/test%s/%s", "data", "family/case.tsv") }`, true},
		{"composed_variable_format", ``, `import "fmt"
var pattern = "../%s/%s"
func consume() string { return fmt.Sprintf(pattern, "support/testdata", "family/case.tsv") }`, true},
		{"composed_variable_component", ``, `import "fmt"
var root = "support/testdata"
func consume() string { return fmt.Sprintf("../%s/%s", root, "family/case.tsv") }`, true},
		{"typed_binding", `func Fixture() string { return "testdata/family/case.tsv" }`, `import s "example/support"
var captured string = s.Fixture()
func consume() string { return captured }`, false},
		{"typed_binding_pointer", `func Fixture() string { return "testdata/family/case.tsv" }`, `import s "example/support"
type label string
var captured label = label(s.Fixture())
func consume() string { return string(captured) }`, false},
		{"typed_binding_group", `func Fixture() string { return "testdata/family/case.tsv" }`, `import s "example/support"
var (
 captured string = s.Fixture()
)
func consume() string { return captured }`, false},
		{"untyped_binding_group", `func Fixture() string { return "testdata/family/case.tsv" }`, `import s "example/support"
var (
 captured = s.Fixture()
)
func consume() string { return captured }`, false},
		{"untyped_multi_binding_group", `func Fixture() string { return "testdata/family/case.tsv" }`, `import s "example/support"
var (
 ignored, captured = "x", s.Fixture()
)
func consume() string { return captured }`, false},
		{"receiver_method", `type Provider struct{}
func (Provider) Fixture() string { return "testdata/family/case.tsv" }`, `import s "example/support"
var provider s.Provider
func consume() string { return provider.Fixture() }`, false},
		{"receiver_pointer", `type Provider struct{}
func (*Provider) Fixture() string { return "testdata/family/case.tsv" }`, `import s "example/support"
func consume() string { provider := &s.Provider{}; return provider.Fixture() }`, false},
		{"multiple_binding", `func Fixture() string { return "testdata/family/case.tsv" }`, `import s "example/support"
var ignored, captured = "x", s.Fixture()
func consume() string { return captured }`, false},
		{"multiple_typed_binding", `func Fixture() string { return "testdata/family/case.tsv" }`, `import s "example/support"
var ignored, captured string = "x", s.Fixture()
func consume() string { return captured }`, false},
		{"anonymous_struct_return", `func Fixture() struct{ Path string } { return struct{ Path string }{Path: "testdata/family/case.tsv"} }`, `import s "example/support"
func consume() any { return s.Fixture() }`, false},
		{"anonymous_interface_return", `type value string
func (v value) String() string { return string(v) }
func Fixture() interface{ String() string } { return value("testdata/family/case.tsv") }`, `import s "example/support"
func consume() any { return s.Fixture() }`, false},
		{"embed_file", `import "embed"
//go:embed testdata/family/case.tsv
var Embedded embed.FS`, `import s "example/support"
func consume() any { return s.Embedded }`, false},
		{"embed_glob", `import "embed"
//go:embed testdata/family/*.tsv
var Embedded embed.FS`, `import s "example/support"
func consume() any { return s.Embedded }`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			run := func(args ...string) {
				t.Helper()
				c := exec.Command(args[0], args[1:]...)
				c.Dir = dir
				if out, err := c.CombinedOutput(); err != nil {
					t.Fatalf("%v: %v %s", args, err, out)
				}
			}
			write := func(p, s string) {
				t.Helper()
				p = filepath.Join(dir, p)
				if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(p, []byte(s), 0644); err != nil {
					t.Fatal(err)
				}
			}
			run("git", "init", "-q")
			run("git", "config", "user.name", "Fixture")
			run("git", "config", "user.email", "fixture@example.invalid")
			write("go.mod", "module example\n\ngo 1.26\n")
			write("support/helper.go", "package support\n"+tc.support+"\n")
			write("support/direct_test.go", `package support
import "testing"
func TestDirect(t *testing.T) { _ = "testdata/family/case.tsv" }
`)
			write("client/helper.go", "package client\n"+tc.client+"\n")
			if strings.HasPrefix(tc.name, "cross_file_") {
				write("client/paths.go", "package client\nvar base = \"../support/\"\nvar fixture = \"testdata/family/case.tsv\"\n")
			}
			if strings.HasPrefix(tc.name, "cross_nested_group") {
				initial := `strings.TrimSpace("other")`
				if strings.HasSuffix(tc.name, "nested") {
					initial = `strings.TrimSpace(strings.TrimSpace("other"))`
				}
				write("client/paths.go", "package client\nimport \"strings\"\nvar (\n other = "+initial+"\n base = \"../support/\"\n fixture = \"testdata/family/case.tsv\"\n)\n")
				write("client/direct_test.go", "package client\nimport \"testing\"\nfunc TestClientDirect(t *testing.T) { _ = \"../support/testdata/family/case.tsv\" }\n")
			}
			if strings.HasPrefix(tc.name, "cross_group_") {
				kind, typ := "var", ""
				if strings.Contains(tc.name, "const") {
					kind = "const"
				}
				if strings.Contains(tc.name, "typed") {
					typ = " string"
				}
				write("client/paths.go", "package client\n"+kind+" (\n base"+typ+" = \"../support/\"\n fixture"+typ+" = \"testdata/family/case.tsv\"\n)\n")
				write("client/direct_test.go", "package client\nimport \"testing\"\nfunc TestClientDirect(t *testing.T) { _ = \"../support/testdata/family/case.tsv\" }\n")
			}
			write("client/consumer_test.go", `package client
import "testing"
func TestConsumer(t *testing.T) { _ = consume() }
`)
			write("support/testdata/family/case.tsv", "old")
			run("git", "add", ".")
			run("git", "commit", "-qm", "base")
			write("support/testdata/family/case.tsv", "new")
			run("git", "add", ".")
			run("git", "commit", "-qm", "fixture")
			carries := filepath.Join(t.TempDir(), "carries")
			if err := os.WriteFile(carries, []byte("[]"), 0644); err != nil {
				t.Fatal(err)
			}
			fake := filepath.Join(t.TempDir(), "go")
			runner := `#!/bin/sh
case "$*" in
 *./support*) printf '%s\n' '{"Action":"pass","Package":"example/support","Test":"TestDirect"}' '{"Action":"pass","Package":"example/support"}';;
 *./client*) case "$*" in
  *TestConsumer*) printf '%s\n' '{"Action":"fail","Package":"example/client","Test":"TestConsumer"}' '{"Action":"fail","Package":"example/client"}'; exit 1;;
  *) printf '%s\n' '{"Action":"pass","Package":"example/client","Test":"TestClientDirect"}' '{"Action":"pass","Package":"example/client"}';;
 esac;;
esac
`
			if err := os.WriteFile(fake, []byte(runner), 0755); err != nil {
				t.Fatal(err)
			}
			script, _ := filepath.Abs("conformance-pairing.py")
			out, err := exec.Command("python3", script, "--repo", dir, "--base", "HEAD^", "--go", fake, "--carries", carries).CombinedOutput()
			if err == nil || (!strings.Contains(string(out), "client/TestConsumer: red without matching") && !(tc.reject && strings.Contains(string(out), "dynamic fixture root"))) {
				t.Fatalf("fixture consumer missed: %v %s", err, out)
			}
		})
	}
}
