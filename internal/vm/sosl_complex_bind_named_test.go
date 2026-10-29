package vm

import (
	"errors"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Native dynamic_{index,call,paren}_* rows in the quoted-numeric-binds oracle
// supply the syntax outcomes. These local named-route twins ensure that the
// anonymous SYSTEM_MODE boundary does not hide an overload-specific bypass.
func TestSOSLComplexBindNamedRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, binding := range []struct{ name, source, message string }{
			{"index", "terms[0]", "expecting a right parentheses, found '['"},
			{"call", "terms.get(0)", "expecting a right parentheses, found '('"},
			{"paren", "(needle)", "Only variable references are allowed in dynamic SOQL/SOSL."},
		} {
			for _, predicate := range []string{"Name IN (1,2)", "Industry IN (1,2)", "Id IN (1,2)", "Industry = 3", "Id = 3"} {
				for _, route := range []string{"query", "query_access", "find", "find_access"} {
					t.Run(api+"/"+binding.name+"/"+predicate+"/"+route, func(t *testing.T) {
						quote := func(value string) string { return "'" + strings.ReplaceAll(value, "'", "\\'") + "'" }
						query := "FIND 'quotednumericnomatch' RETURNING Account(Id WHERE " + predicate + " AND Name = :" + binding.source + ")"
						call := "Search." + strings.Split(route, "_")[0] + "(" + quote(query)
						if strings.HasSuffix(route, "_access") {
							call += ",AccessLevel.SYSTEM_MODE"
						}
						call += ")"
						source := "String needle='quotednumericnomatch'; List<String> terms=new List<String>{needle}; String observed='accepted'; try { " + call + "; } catch(Exception e) { observed=e.getMessage(); } System.assertEquals(" + quote(binding.message) + ",observed);"
						body, err := CompileAnonymous(source)
						if err != nil {
							t.Fatal(err)
						}
						machine := New(nil)
						org := storage.NewOrgState()
						storage.EnsureStandardObject(&org, "Account")
						machine.SetOrg(&org)
						if err = machine.RegisterClass(Class{Name: "QuotedBindProbe"}); err != nil {
							t.Fatal(err)
						}
						if err = machine.RegisterMethod(Method{Name: "QuotedBindProbe.run", ClassName: "QuotedBindProbe", IsStatic: true, Access: "public", APIVersion: api, Program: body}); err != nil {
							t.Fatal(err)
						}
						driver, err := CompileAnonymous("QuotedBindProbe.run();")
						if err != nil {
							t.Fatal(err)
						}
						_, err = machine.Execute(driver)
						if binding.name == "paren" {
							var terminal *RuntimeError
							if !errors.As(err, &terminal) || terminal.Type != "" || terminal.ExceptionMessage() != binding.message {
								t.Fatalf("terminal=%v", err)
							}
							return
						}
						if err != nil {
							t.Fatal(err)
						}
					})
				}
			}
		}
	}
}
