package sema

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// Owned rejection rows from the Integer/Long, Double/Decimal, String and
// collection oracles have identical native wording at API 62 and 67.
func TestValueCollectionDiagnosticWording(t *testing.T) {
	rows := []struct{ id, source, want string }{
		{"Integer and Long/K04", "Integer i = 7 % 2; ", "Found punctuation symbol or operator '%' that isn't valid in Apex."},
		{"Integer and Long/K13", "Integer a = 5; Integer b = 5; Boolean r = a === b; ", "Exact equality operator only allowed for reference types: Integer"},
		{"Integer and Long/K17", "Integer i = (Integer) '5'; ", "Incompatible types since an instance of String is never an instance of Integer"},
		{"Integer and Long/K19", "Object r = 6 & 3 == 2; ", "& operator can only be applied to Boolean expressions or to Integer or Long expressions"},
		{"Integer and Long/K23", "Integer i = null + 1; ", "Arithmetic expressions must use numeric arguments"},
		{"Integer and Long/Z01", "Long x = 9223372036854775808L; ", "Illegal long"},
		{"Integer and Long/Z02", "Long x = -9223372036854775808L; ", "Illegal long"},
		{"Double and Decimal/K08", "Object r; Decimal d = 1.0; r = d % 2; ", "Found punctuation symbol or operator '%' that isn't valid in Apex."},
		{"Double and Decimal/K09", "Object r; Double d = 1.0; r = d & 2; ", "& operator can only be applied to Boolean expressions or to Integer or Long expressions"},

		{"Integer and Long/K06", "Integer i = 5L; ", "Illegal assignment from Long to Integer"},
		{"Integer and Long/K07", "Integer i = 1.0; ", "Illegal assignment from Decimal to Integer"},
		{"Integer and Long/K08", "Integer i = 5; i += 1.5; ", "Illegal assignment from Decimal to Integer"},
		{"Integer and Long/K21", "Integer i = 5; i = i / 2.0; ", "Illegal assignment from Decimal to Integer"},
		{"Integer and Long/K22", "Long l = 5; Integer i = l; ", "Illegal assignment from Long to Integer"},
		{"Integer and Long/K26", "Integer i = true ? 1 : 2L; ", "Illegal assignment from Long to Integer"},
		{"Integer and Long/K27", "Integer i = 2147483647L; ", "Illegal assignment from Long to Integer"},
		{"Integer and Long/K29", "Integer i = 5; Integer j = i.intValue(); ", "Method does not exist or incorrect signature: void intValue() from the type Integer"},
		{"Integer and Long/K30", "Integer i = 5; Long j = i.longValue(); ", "Method does not exist or incorrect signature: void longValue() from the type Integer"},
		{"Integer and Long/K31", "Map<Integer, String> m = new Map<Integer, String>{1 => 'a'}; String s = m.get(1L); ", "Method does not exist or incorrect signature: void get(Long) from the type Map<Integer,String>"},
		{"Integer and Long/K32", "Boolean b = 1; ", "Illegal assignment from Integer to Boolean"},
		{"Integer and Long/Z03", "Long x = 1.0; ", "Illegal assignment from Decimal to Long"},
		{"Integer and Long/Z04", "Long x = 5L; Integer y = x; ", "Illegal assignment from Long to Integer"},
		{"Integer and Long/Z05", "Long x = 1L; x += 0.5; ", "Illegal assignment from Decimal to Long"},
		{"Double and Decimal/E04", "Object r; Decimal d = 1.0; r = d.compareTo(2.0);", "Method is not visible: Integer Decimal.compareTo(Decimal)"},
		{"Double and Decimal/E05", "Object r; Double d = 1.0; r = d.compareTo(2.0);", "Method does not exist or incorrect signature: void compareTo(Decimal) from the type Double"},
		{"Double and Decimal/F04", "Object r; Double d = -1234.567; r = d.abs();", "Method does not exist or incorrect signature: void abs() from the type Double"},
		{"Double and Decimal/K03", "Object r; Integer i = 1.0; ", "Illegal assignment from Decimal to Integer"},
		{"Double and Decimal/K04", "Object r; Long i = 1.0; ", "Illegal assignment from Decimal to Long"},
		{"Double and Decimal/K10", "Object r; Double d = 1.0; r = d.setScale(2); ", "Method does not exist or incorrect signature: void setScale(Integer) from the type Double"},
		{"Double and Decimal/K11", "Object r; Double d = 1.0; r = d.doubleValue(); ", "Method does not exist or incorrect signature: void doubleValue() from the type Double"},
		{"Double and Decimal/K12", "Object r; Decimal d = 1.0; r = d.floatValue(); ", "Method does not exist or incorrect signature: void floatValue() from the type Decimal"},
		{"Double and Decimal/V17", "Decimal.valueOf(null);", "Ambiguous method signature: void valueOf(NULL)"},
		{"Double and Decimal/V34", "Double.valueOf(null);", "Ambiguous method signature: void valueOf(NULL)"},
		{"String/K01", "Object r = 'abc'.charAt('1'); ", "Method does not exist or incorrect signature: void charAt(String) from the type String"},
		{"String/K02", "Object r = 'abc'.substring(); ", "Method does not exist or incorrect signature: void substring() from the type String"},
		{"String/K03", "Object r = String.isBlank(1); ", "Method does not exist or incorrect signature: void isBlank(Integer) from the type String"},
		{"String/K04", "Object r = 'abc'.containsValue('a'); ", "Method does not exist or incorrect signature: void containsValue(String) from the type String"},
		{"String/K05", "Object r = String.fromCharArray(new List<String>{'a'}); ", "Method does not exist or incorrect signature: void fromCharArray(List<String>) from the type String"},
		{"String/K06", "Object r = 'abc'.getChars(0); ", "Method does not exist or incorrect signature: void getChars(Integer) from the type String"},
		{"collections/C001", "List<Integer> x=new List<Integer>{'a'};", "Initial expression is of incorrect type, expected: Integer but was: String"},
		{"collections/C002", "List<Integer> x=new List<Integer>(); x.add(true);", "Method does not exist or incorrect signature: void add(Boolean) from the type List<Integer>"},
		{"collections/C003", "List<Integer> x=new List<Integer>(); x.get();", "Method does not exist or incorrect signature: void get() from the type List<Integer>"},
		{"collections/C004", "List<Integer> x=new List<Integer>(); x.set(0);", "Method does not exist or incorrect signature: void set(Integer) from the type List<Integer>"},
		{"collections/C005", "Set<Integer> x=new Set<Integer>(); x.get(0);", "Method does not exist or incorrect signature: void get(Integer) from the type Set<Integer>"},
		{"collections/C006", "Set<Integer> x=new Set<Integer>(); x.add(0,1);", "Method does not exist or incorrect signature: void add(Integer, Integer) from the type Set<Integer>"},
		{"collections/C007", "Map<String,Integer> x=new Map<String,Integer>(); x.put(1,1);", "Method does not exist or incorrect signature: void put(Integer, Integer) from the type Map<String,Integer>"},
		{"collections/C008", "Map<String,Integer> x=new Map<String,Integer>(); x.put('a','b');", "Method does not exist or incorrect signature: void put(String, String) from the type Map<String,Integer>"},
		{"collections/C009", "List<String> x=new List<Integer>();", "Illegal assignment from List<Integer> to List<String>"},
		{"collections/C010", "Set<String> x=new Set<Integer>();", "Illegal assignment from Set<Integer> to Set<String>"},
		{"collections/C011", "Map<String,String> x=new Map<String,Integer>();", "Illegal assignment from Map<String,Integer> to Map<String,String>"},
		{"collections/C013", "Set<Integer> x=new Set<Integer>(); String y=x.add(1);", "Illegal assignment from Boolean to String"},
		{"collections/C014", "List<Integer> x=new List<Integer>(); x.remove(0,1);", "Method does not exist or incorrect signature: void remove(Integer, Integer) from the type List<Integer>"},
		{"collections/C015", "Map<String,Integer> x=new Map<String,Integer>(); x.containsAll(new List<String>());", "Method does not exist or incorrect signature: void containsAll(List<String>) from the type Map<String,Integer>"},
		{"collections/C016", "Object r; Map<Integer,String> x=new Map<Integer,String>(); r=x.get(1L);", "Method does not exist or incorrect signature: void get(Long) from the type Map<Integer,String>"},
		{"collections/R056", "Object r; Map<String,Integer> x = new Map<String,Integer>{'a'=>1,'b'=>2}; r=x.containsValue(1);", "Method does not exist or incorrect signature: void containsValue(Integer) from the type Map<String,Integer>"},
		{"collections/R063", "Object r; Map<String,Integer> x = new Map<String,Integer>{'a'=>1,'b'=>2}; x.put('n',null); r=x.containsValue(null);", "Method does not exist or incorrect signature: void containsValue(NULL) from the type Map<String,Integer>"},
		{"collections/R125", "Object r; Map<String,String> x = new Map<String,String>{'a'=>'a','b'=>'b'}; r=x.containsValue('a');", "Method does not exist or incorrect signature: void containsValue(String) from the type Map<String,String>"},
		{"collections/R132", "Object r; Map<String,String> x = new Map<String,String>{'a'=>'a','b'=>'b'}; x.put('n',null); r=x.containsValue(null);", "Method does not exist or incorrect signature: void containsValue(NULL) from the type Map<String,String>"},
		{"collections/R194", "Object r; Map<String,Decimal> x = new Map<String,Decimal>{'a'=>1.0,'b'=>2.0}; r=x.containsValue(1.0);", "Method does not exist or incorrect signature: void containsValue(Decimal) from the type Map<String,Decimal>"},
		{"collections/R201", "Object r; Map<String,Decimal> x = new Map<String,Decimal>{'a'=>1.0,'b'=>2.0}; x.put('n',null); r=x.containsValue(null);", "Method does not exist or incorrect signature: void containsValue(NULL) from the type Map<String,Decimal>"},
	}
	for _, row := range rows {
		for _, version := range []string{"62.0", "67.0"} {
			for _, route := range []string{"anonymous", "named"} {
				t.Run(row.id+"/"+version+"/"+route, func(t *testing.T) {
					var result Result
					if route == "anonymous" {
						result = AnalyzeAnonymous(typesys.Index{}, row.source, version)
					} else {
						root := t.TempDir()
						file := filepath.Join(root, "Probe.cls")
						source := "@IsTest private class Probe { static void observed() {" + row.source + "} }"
						if err := os.WriteFile(file, []byte(source), 0600); err != nil {
							t.Fatal(err)
						}
						result = Analyze(typesys.Build(project.Project{Root: root, SourceAPIVersion: version, ApexFiles: []string{file}}, schema.Schema{}))
					}
					for _, d := range result.Diagnostics {
						if d.Severity != diagnostic.Error {
							continue
						}
						text := d.NativeMessage
						if text == "" {
							text = d.Message
						}
						if text != row.want {
							t.Fatalf("first error = %q, want native %q; diagnostics=%#v", text, row.want, result.Diagnostics)
						}
						return
					}
					t.Fatalf("want native %q, got %#v", row.want, result.Diagnostics)
				})
			}
		}
	}
}
