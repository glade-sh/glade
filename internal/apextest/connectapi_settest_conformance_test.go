package apextest

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// connectapi_settest/org.tsv holds one native @IsTest row per class and API
// version. Every method ends in System.assert(false, observation), so the
// native method message carries the observed behavior.
func TestConnectAPISetTestRegistrationMatchesNative(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "connectapi_settest")
	raw, err := os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	classOf := map[string]string{}
	cases, err := os.ReadFile(filepath.Join(dir, "cases.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(cases)), "\n")[1:] {
		f := strings.Split(line, "\t")
		classOf[f[0]] = f[2]
	}
	entry := regexp.MustCompile(`(?:^|; )([a-z0-9_]+)=(Fail|Pass) `)
	expected := map[string]map[string]string{} // class -> method -> message
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n")[1:] {
		f := strings.SplitN(line, "\t", 6)
		class := classOf[f[0]]
		if class == "" || len(f) < 6 {
			t.Fatalf("bad org row %q", f[0])
		}
		out := f[5]
		locs := entry.FindAllStringSubmatchIndex(out, -1)
		expected[class] = map[string]string{}
		for i, loc := range locs {
			end := len(out)
			if i+1 < len(locs) {
				end = locs[i+1][0]
			}
			expected[class][out[loc[2]:loc[3]]] = strings.TrimSpace(out[loc[1]:end])
		}
	}
	for _, api := range []string{"62.0", "67.0"} {
		p := newSalesforceSurfaceProject(t, `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
		src := filepath.Join(dir, "project", "force-app", "main", "default", "classes")
		entries, err := os.ReadDir(src)
		if err != nil {
			t.Fatal(err)
		}
		suffix := "A" + strings.TrimSuffix(api, ".0")
		for _, e := range entries {
			if !strings.Contains(e.Name(), suffix) || !strings.HasSuffix(e.Name(), ".cls") {
				continue
			}
			body, err := os.ReadFile(filepath.Join(src, e.Name()))
			if err != nil {
				t.Fatal(err)
			}
			p.writeClass(strings.TrimSuffix(e.Name(), ".cls"), string(body))
		}
		run := p.run()
		got := map[string]map[string]string{}
		for _, suite := range run.Suites {
			for _, c := range suite.Cases {
				if got[c.ClassName] == nil {
					got[c.ClassName] = map[string]string{}
				}
				msg := "<no problem>"
				if c.Problem != nil {
					msg = strings.TrimSpace(c.Problem.Type + ": " + c.Problem.Message)
				}
				got[c.ClassName][strings.ToLower(c.MethodName)] = msg
			}
		}
		for class, methods := range expected {
			if !strings.HasSuffix(class, suffix) {
				continue
			}
			for method, want := range methods {
				if connectAPISetTestParked.MatchString(method) {
					continue
				}
				have := got[class][method]
				if !connectAPISetTestMessageMatches(have, want) {
					t.Errorf("%s %s.%s\n  native: %s\n  glade:  %s", api, class, method, want, have)
				}
			}
		}
	}
}

// connectAPISetTestParked lists rows that need a real ConnectApi service call
// (addFavorite creates a FeedFavorite; getFlagsOnFeedElement reaches the org).
// Services stay unsupported, so these rows are not asserted.
var connectAPISetTestParked = regexp.MustCompile(`^c7[a-e]_|^c7i_`)

// The native CPU delta is a timing measurement.
var connectAPISetTestCPU = regexp.MustCompile(`dCpu=\d+`)

func connectAPISetTestMessageMatches(have, want string) bool {
	have = connectAPISetTestCPU.ReplaceAllString(have, "dCpu=N")
	want = connectAPISetTestCPU.ReplaceAllString(want, "dCpu=N")
	return have == want
}
