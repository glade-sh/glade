package apextest

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// capi4_output_json/org.tsv: for every ConnectApi type name, the string that
// JSON.serialize(Type.forName(name).newInstance()) returns natively inside an
// @IsTest method, or X:<exception type>:<message> when Type.forName,
// newInstance or JSON.serialize throws. Glade must return the same string at
// class APIs 62.0, 67.0 and 68.0: the key set and order, null handling,
// defaults, the JSON text null and [], and the failures.
//
// One boundary: a service class the sweep org did not enable answers
// "Type is not visible: <name>" natively. Where the service is visible the same
// call reports that the class has no no-arg constructor, which is what glade
// answers for a project that declares no org shape; the feature gate itself is
// pinned by capi3_feature_gates.
var capi4UUID = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)

func TestCapi4OutputJSONSweep(t *testing.T) {
	f, err := os.Open(filepath.Join("testdata", "conformance", "capi4_output_json", "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	apis := []string{"62.0", "67.0", "68.0"}
	var names []string
	expected := map[string]map[string]string{}
	for _, api := range apis {
		expected[api] = map[string]string{}
	}
	scan := bufio.NewScanner(f)
	scan.Buffer(make([]byte, 1<<20), 1<<20)
	for scan.Scan() {
		p := strings.Split(scan.Text(), "\t")
		if len(p) != 4 || p[0] == "name" {
			continue
		}
		names = append(names, p[0])
		for i, api := range apis {
			want := p[1+i]
			if strings.HasPrefix(want, "X:System.TypeException:Type is not visible: ") {
				want = "X:System.TypeException:" + p[0] + " does not have a no-arg constructor"
			}
			expected[api][p[0]] = want
		}
	}
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
	if len(names) != 4749 {
		t.Fatalf("native rows = %d, want 4749", len(names))
	}
	for _, api := range apis {
		t.Run(api, func(t *testing.T) {
			p := newSalesforceSurfaceProject(t, `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			classes := 0
			for start := 0; start < len(names); start += 100 {
				end := min(start+100, len(names))
				var b strings.Builder
				fmt.Fprintf(&b, "@IsTest private class Sweep%d {\n", classes)
				b.WriteString(` private static String s(String n){ try{ return n+'\t'+JSON.serialize(Type.forName(n).newInstance()); }catch(Exception e){ return n+'\tX:'+e.getTypeName()+':'+(e.getMessage()==null?'':e.getMessage().replace('\n',' ')); } }` + "\n")
				for m, ms := 0, start; ms < end; m, ms = m+1, ms+10 {
					me := min(ms+10, end)
					items := make([]string, 0, me-ms)
					for _, name := range names[ms:me] {
						items = append(items, "s('"+name+"')")
					}
					fmt.Fprintf(&b, " @IsTest static void t%d(){ System.assert(false,'ROW\\n'+String.join(new List<String>{%s},'\\n')); }\n", m, strings.Join(items, ","))
				}
				b.WriteString("}\n")
				p.writeClass(fmt.Sprintf("Sweep%d", classes), b.String())
				writeFile(t, filepath.Join(p.root, fmt.Sprintf("force-app/main/classes/Sweep%d.cls-meta.xml", classes)), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
				classes++
			}
			run := p.run()
			got := map[string]string{}
			for _, suite := range run.Suites {
				for _, testCase := range suite.Cases {
					if testCase.Problem == nil {
						t.Fatalf("%s.%s passed; every sweep method ends in a failing assert", suite.Name, testCase.MethodName)
					}
					lines := strings.Split(testCase.Problem.Message, "\n")
					if len(lines) < 2 || !strings.HasSuffix(lines[0], "ROW") {
						t.Fatalf("%s.%s: unexpected problem %q", suite.Name, testCase.MethodName, testCase.Problem.Message)
					}
					for _, line := range lines[1:] {
						name, result, _ := strings.Cut(line, "\t")
						got[name] = capi4UUID.ReplaceAllString(result, "<uuid>")
					}
				}
			}
			var bad []string
			for _, name := range names {
				want := capi4UUID.ReplaceAllString(expected[api][name], "<uuid>")
				if got[name] != want {
					bad = append(bad, fmt.Sprintf("%s\n  native %.200s\n  glade  %.200s", name, want, got[name]))
				}
			}
			if len(bad) > 0 {
				show := bad
				if len(show) > 12 {
					show = show[:12]
				}
				t.Fatalf("%d of %d types differ from native at API %s:\n%s", len(bad), len(names), api, strings.Join(show, "\n"))
			}
		})
	}
}
