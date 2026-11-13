package dml

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

const surnameNativeDir = "testdata/conformance/surname_keyboard/"

func surnameTable(t *testing.T, dir, name string) []map[string]string {
	t.Helper()
	f, err := os.Open(dir + name)
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
	var out []map[string]string
	for _, row := range rows[1:] {
		m := map[string]string{}
		for i, k := range rows[0] {
			m[k] = row[i]
		}
		out = append(out, m)
	}
	return out
}

// Exact residuals are deliberately keyed by the input, never used by product
// code. Membership and both scorers remain strict even on these grade rows.
var surnameKnownExactDifferences = map[string][2]int{
	"Ikir/Ikirt": {99, 98},
	"Ikirt/Ikir": {99, 98},
	"Jlun/Jlunm": {97, 98},
	"Jlun/Jlunq": {99, 98},
	"Jlunm/Jlun": {97, 98},
	"Jlunq/Jlun": {99, 98},
	"Riim/Riimq": {99, 98},
	"Riimq/Riim": {99, 98},
	"Rume/Rumer": {99, 98},
	"Rumer/Rume": {99, 98},
	"Ssxb/Ssxbg": {99, 98},
	"Ssxbg/Ssxb": {99, 98},
	"Sznc/Szncd": {99, 98},
	"Szncd/Sznc": {99, 98},
	"Zela/Zelak": {99, 98},
	"Zela/Zelaq": {99, 98},
	"Zelak/Zela": {99, 98},
	"Zelaq/Zela": {99, 98},
}

// NATIVE surname_keyboard/org.tsv:2 anonymous 62.0; all APIs, routes and repeats
// are checked, including the Lead mirrors and corroboration negative controls.
func TestFuzzySurnameNativeRows(t *testing.T) {
	surnameNativeRows(t, surnameNativeDir, []string{"pairs1.tsv", "pairs1b.tsv", "pairs2.tsv"}, 496, 62, surnameKnownExactDifferences)
}

// NATIVE first, middle and final substitutions at both APIs and routes:
// testdata/conformance/surname_substitution/org.tsv. Each method repeats twice.
func TestFuzzySurnameSubstitutionNativeRows(t *testing.T) {
	surnameNativeRows(t, "testdata/conformance/surname_substitution/", []string{"pairs.tsv"}, 512, 64, nil)
}

func surnameNativeRows(t *testing.T, dir string, tables []string, observations, methods int, residuals map[string][2]int) {
	t.Helper()
	t.Setenv("GLADE_STANDARD_DUPLICATE_RULES", "on")
	inputs := map[string][]map[string]string{}
	for _, name := range tables {
		for _, p := range surnameTable(t, dir, name) {
			inputs["m_"+p["method"]] = append(inputs["m_"+p["method"]], p)
		}
	}
	type counts struct{ total, exact, membership, falseMatches int }
	panels := map[string]*counts{}
	rows := surnameTable(t, dir, "org.tsv")
	if len(rows) != observations || len(inputs) != methods {
		t.Fatalf("matrix: %d observations, %d methods", len(rows), len(inputs))
	}
	seenResidual := map[string]bool{}
	seenRows := map[string]bool{}
	for _, row := range rows {
		key := row["method"] + "/" + row["api"] + "/" + row["route"] + "/" + row["repeat"]
		if seenRows[key] || (row["api"] != "62.0" && row["api"] != "67.0") || (row["route"] != "anonymous" && row["route"] != "istest") || (row["repeat"] != "1" && row["repeat"] != "2") {
			t.Fatalf("invalid or duplicate observation %s", key)
		}
		seenRows[key] = true
		pairs := inputs[row["method"]]
		if len(pairs) == 0 {
			t.Fatalf("missing inputs: %v", row)
		}
		observation := strings.Split(row["observation"], "|")
		if len(observation) != 3 {
			t.Fatal(row)
		}
		entries := strings.Split(observation[2], ",")
		for _, p := range pairs {
			index, err := strconv.Atoi(p["index"])
			if err != nil {
				t.Fatal(err)
			}
			entry := strings.SplitN(entries[index], ":", 2)[1]
			find, save, _ := strings.Cut(entry, "/")
			wantMatch := find != "-"
			want := 0
			if wantMatch {
				grade := strings.Split(strings.Split(find, "=")[1], "~")
				want, err = strconv.Atoi(grade[0])
				if err != nil || grade[1] != fmt.Sprintf("%d.0", want) {
					t.Fatal(find)
				}
			}
			if wantMatch && save != find || !wantMatch && save != "S" {
				t.Fatalf("Find/DML disagree: %s", entry)
			}
			panel := p["panel"]
			if panels[panel] == nil {
				panels[panel] = &counts{}
			}
			c := panels[panel]
			c.total++
			record := func(last string, candidate bool) storage.Record {
				fields := map[string]storage.Value{"FirstName": storage.StringValue("Brannock"), "LastName": storage.StringValue(last), "Email": storage.StringValue("s@k.example")}
				switch p["variant"] {
				case "noemail", "phone":
					delete(fields, "Email")
					if p["variant"] == "phone" {
						fields["Phone"] = storage.StringValue("(630) 555-1234")
					}
				case "diffemail":
					if candidate {
						fields["Email"] = storage.StringValue("c@z.example")
					}
				case "diffname":
					if candidate {
						fields["FirstName"] = storage.StringValue("Ormerod")
					}
				}
				return storage.Record{Object: p["obj"], Fields: fields}
			}
			candidate, existing := record(p["cand"], true), record(p["seed"], false)
			match, score := standardDuplicateMatcher(nil, p["obj"], p["obj"], candidate)(existing)
			direct, dscore := standardPersonMatch(strings.ToLower(p["obj"]), standardPersonOf(nil, p["obj"], candidate), standardPersonOf(nil, p["obj"], existing))
			got := int(score + 0.5)
			if match == wantMatch {
				c.membership++
			}
			if match && !wantMatch {
				c.falseMatches++
			}
			if match == wantMatch && (!match || got == want) {
				c.exact++
			}
			key := p["seed"] + "/" + p["cand"]
			expected := want
			if diff, ok := residuals[key]; ok && panel == "A" {
				if want != diff[0] {
					t.Fatalf("stale native residual %s", key)
				}
				expected = diff[1]
				seenResidual[key] = true
			}
			if match != wantMatch || match && got != expected || direct != match || match && int(dscore+0.5) != got {
				t.Errorf("%s/%s/%s/%s %s %s: native=%v/%d got=%v/%d direct=%v/%.0f expected=%d", row["api"], row["route"], row["repeat"], row["method"], p["index"], key, wantMatch, want, match, got, direct, dscore, expected)
			}
		}
	}
	for panel, c := range panels {
		t.Logf("PANEL %s exact=%d/%d membership=%d/%d false_matches=%d", panel, c.exact, c.total, c.membership, c.total, c.falseMatches)
	}
	for key := range residuals {
		if !seenResidual[key] {
			t.Errorf("unused residual %s", key)
		}
	}
}

// Local contract: unmeasured eight-letter swap geometries keep the inherited
// keyboard gate. The native five-column boundary must not admit wider swaps.
func TestFuzzySurnameUnmeasuredSwapKeepsKeyboardGate(t *testing.T) {
	const a, b = "Gklnjdpq", "Gklnjdqp"
	if lastNameCode(foldText(a)) != lastNameCode(foldText(b)) || lastNameMetaphone3Equal(foldText(a), foldText(b)) {
		t.Fatal("control must reach the ordinary same-key keyboard branch")
	}
	score, eligible := keyboardSimilarity(foldText(a), foldText(b))
	if eligible || score >= 90 {
		t.Fatal("control must miss the ordinary keyboard gate")
	}
	if match, confidence := FuzzyScore("LastName", a, b, false); match {
		t.Fatalf("unmeasured wide swap bypasses the ordinary keyboard gate: %.2f", confidence)
	}
}

// Local contract: the new panel measures lengths 4 to 8. Three-letter
// nonterminal/cross-row substitutions and other far-key geometries retain the
// inherited surname gate; they cannot gain supplementary membership.
func TestFuzzySurnameUnmeasuredSubstitutionKeepsKeyboardGate(t *testing.T) {
	for _, pair := range [][2]string{
		{"Mtq", "Ntq"}, {"Qma", "Qna"}, {"Rfq", "Rfa"},
		{"Qskjh", "Uskjh"}, {"Askjh", "Jskjh"},
	} {
		t.Run(pair[0]+"/"+pair[1], func(t *testing.T) {
			a, b := strings.ToLower(pair[0]), strings.ToLower(pair[1])
			if lastNameMetaphone3Equal(a, b) || lastNameCode(a) == lastNameCode(b) {
				t.Fatal("control must reach the supplementary surname path")
			}
			if match, confidence := standardCorroboratedSurname(a, b); match {
				t.Fatalf("unmeasured substitution bypasses the inherited gate: %.2f", confidence)
			}
		})
	}
}
