package sema

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// splitReceiverFixtureShapes renders each captured statement's canonical
// shape for the split call it contains.
func splitReceiverFixtureShapes(t *testing.T) (rejected, accepted map[string]string) {
	t.Helper()
	const fixture = "testdata/receiver_split"
	f, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	rejected, accepted = map[string]string{}, map[string]string{}
	for _, row := range rows[1:] {
		body, err := os.ReadFile(filepath.Join(fixture, "apex", row[0]+".apex"))
		if err != nil {
			t.Fatal(err)
		}
		source := string(body)
		pos := strings.LastIndex(strings.TrimSuffix(source, ";\n"), ";\n") + 1
		for pos < len(source) && source[pos] == '\n' {
			pos++
		}
		tokens, ok := splitReceiverStatement(source, pos)
		if !ok {
			t.Fatalf("%s: statement did not lex", row[0])
		}
		for _, call := range [][3]string{{"ReceiverProbe", "value", ""}, {"System.String", "valueOf", ""}, {"x", "instanceValue", "\x00local"}, {"obj", "instanceValue", "\x00local"}} {
			shape, _, found := splitReceiverShape(tokens, call[0], call[1])
			if !found {
				continue
			}
			if row[3] == "true" {
				accepted[shape+call[2]] = row[0]
			} else {
				rejected[shape+call[2]] = row[0]
			}
		}
	}
	return rejected, accepted
}

// The rejection table holds exactly the rejected captures' shapes plus the
// synthetic equivalent of the supplied chain-recv rows, and no accepted one.
func TestReceiverSplitShapesMatchCaptures(t *testing.T) {
	rejected, accepted := splitReceiverFixtureShapes(t)
	rejected["R\n.M();"] = "supplied chain-recv"
	var missing, extra []string
	for shape, id := range rejected {
		if _, ok := splitReceiverShapes[shape]; !ok {
			missing = append(missing, id+" "+shape)
		}
	}
	for shape := range splitReceiverShapes {
		if _, ok := rejected[shape]; !ok {
			extra = append(extra, shape)
		}
	}
	for shape, id := range accepted {
		if _, ok := splitReceiverShapes[shape]; ok {
			extra = append(extra, "accepted "+id+" "+shape)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	if len(missing) > 0 || len(extra) > 0 {
		t.Fatalf("shape table differs from captures:\nmissing %q\nextra %q", missing, extra)
	}
}
