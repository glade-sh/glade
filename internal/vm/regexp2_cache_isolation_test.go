package vm

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

// The process cache owns compiled, input-independent plans only. Matchers,
// search positions, captures and regions belong to their VM values, across
// independent holders, executions, org installations and sibling VMs.
func TestRegexp2CachedPlansKeepMatcherStatePrivate(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			execute := func(machine *VM, source string) {
				t.Helper()
				program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
			}
			first := New(nil)
			firstOrg := storage.NewOrgState()
			first.SetOrg(&firstOrg)
			execute(first, `Matcher saved = Pattern.compile('(a+)(b)').matcher('aaab ab'); System.assert(saved.find()); System.assertEquals('aaa',saved.group(1)); System.assertEquals(0,saved.start());`)
			for _, namespace := range []string{"", "owner", "foreign"} {
				other := New(nil)
				org := storage.NewOrgState()
				org.Namespace = namespace
				other.SetOrg(&org)
				execute(other, `Matcher m = Pattern.compile('(a+)(b)').matcher('ab aaab'); Matcher sibling=Pattern.compile('(a+)(b)').matcher('aab'); System.assert(m.find()); System.assertEquals('a',m.group(1)); System.assert(sibling.find()); System.assertEquals('aa',sibling.group(1)); System.assert(m.find()); System.assertEquals('aaa',m.group(1)); System.assertEquals('aa',sibling.group(1));`)
				// New org installation keeps existing Apex variables; their mutable
				// matcher state must remain owned while the compiled plan stays reusable.
				replacement := storage.NewOrgState()
				replacement.Namespace = namespace
				first.SetOrg(&replacement)
				execute(first, `System.assertEquals('aaa',saved.group(1)); System.assertEquals(0,saved.start());`)
			}
			execute(first, `System.assert(saved.find()); System.assertEquals('a',saved.group(1)); System.assertEquals(5,saved.start());`)
		})
	}
}
