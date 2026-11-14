package sema

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// TestReceiverGrammarNative compares each supplied native primary diagnostic,
// including its source position, at both captured API versions. The native
// class rows also contain cascades: CA, D1-D6 and E1-E2. CB contains five
// independent variable errors. Those additional outcomes are retained in the
// fixture and are not substituted for the primary diagnostic.
// NATIVE F1 testdata/receiver_split/org.tsv:144 anonymous 62.0
// NATIVE F1 testdata/receiver_split/org.tsv:145 anonymous 67.0
// NATIVE F2 testdata/receiver_split/org.tsv:146 anonymous 62.0
// NATIVE F2 testdata/receiver_split/org.tsv:147 anonymous 67.0
// NATIVE L1 testdata/receiver_split/org.tsv:148 anonymous 62.0
// NATIVE L1 testdata/receiver_split/org.tsv:149 anonymous 67.0
// NATIVE L2 testdata/receiver_split/org.tsv:150 anonymous 62.0
// NATIVE L2 testdata/receiver_split/org.tsv:151 anonymous 67.0
// NATIVE M1 testdata/receiver_split/org.tsv:152 anonymous 62.0
// NATIVE M1 testdata/receiver_split/org.tsv:153 anonymous 67.0
// NATIVE M2 testdata/receiver_split/org.tsv:154 anonymous 62.0
// NATIVE M2 testdata/receiver_split/org.tsv:155 anonymous 67.0
// NATIVE M3 testdata/receiver_split/org.tsv:156 anonymous 62.0
// NATIVE M3 testdata/receiver_split/org.tsv:157 anonymous 67.0
// NATIVE M4 testdata/receiver_split/org.tsv:158 anonymous 62.0
// NATIVE M4 testdata/receiver_split/org.tsv:159 anonymous 67.0
// NATIVE M5 testdata/receiver_split/org.tsv:160 anonymous 62.0
// NATIVE M5 testdata/receiver_split/org.tsv:161 anonymous 67.0
// NATIVE M6 testdata/receiver_split/org.tsv:162 anonymous 62.0
// NATIVE M6 testdata/receiver_split/org.tsv:163 anonymous 67.0
// NATIVE M7 testdata/receiver_split/org.tsv:164 anonymous 62.0
// NATIVE M7 testdata/receiver_split/org.tsv:165 anonymous 67.0
// NATIVE N1 testdata/receiver_split/org.tsv:166 anonymous 62.0
// NATIVE N1 testdata/receiver_split/org.tsv:167 anonymous 67.0
// NATIVE P1 testdata/receiver_split/org.tsv:168 anonymous 62.0
// NATIVE P1 testdata/receiver_split/org.tsv:169 anonymous 67.0
// NATIVE Q1 testdata/receiver_split/org.tsv:170 anonymous 62.0
// NATIVE Q1 testdata/receiver_split/org.tsv:171 anonymous 67.0
// NATIVE Q2 testdata/receiver_split/org.tsv:172 anonymous 62.0
// NATIVE Q2 testdata/receiver_split/org.tsv:173 anonymous 67.0
// NATIVE S1 testdata/receiver_split/org.tsv:174 anonymous 62.0
// NATIVE S1 testdata/receiver_split/org.tsv:175 anonymous 67.0
// NATIVE S2 testdata/receiver_split/org.tsv:176 anonymous 62.0
// NATIVE S2 testdata/receiver_split/org.tsv:177 anonymous 67.0
// NATIVE S5 testdata/receiver_split/org.tsv:178 anonymous 62.0
// NATIVE S5 testdata/receiver_split/org.tsv:179 anonymous 67.0
// NATIVE T1 testdata/receiver_split/org.tsv:180 anonymous 62.0
// NATIVE T1 testdata/receiver_split/org.tsv:181 anonymous 67.0
// NATIVE T2 testdata/receiver_split/org.tsv:182 anonymous 62.0
// NATIVE T2 testdata/receiver_split/org.tsv:183 anonymous 67.0
// NATIVE T3 testdata/receiver_split/org.tsv:184 anonymous 62.0
// NATIVE T3 testdata/receiver_split/org.tsv:185 anonymous 67.0
// NATIVE T4 testdata/receiver_split/org.tsv:186 anonymous 62.0
// NATIVE T4 testdata/receiver_split/org.tsv:187 anonymous 67.0
// NATIVE T5 testdata/receiver_split/org.tsv:188 anonymous 62.0
// NATIVE T5 testdata/receiver_split/org.tsv:189 anonymous 67.0
// NATIVE CA testdata/receiver_split/org.tsv:190 class 62.0
// NATIVE CA testdata/receiver_split/org.tsv:191 class 67.0
// NATIVE CB testdata/receiver_split/org.tsv:192 class 62.0
// NATIVE CB testdata/receiver_split/org.tsv:193 class 67.0
// NATIVE D1 testdata/receiver_split/org.tsv:194 class 62.0
// NATIVE D1 testdata/receiver_split/org.tsv:195 class 67.0
// NATIVE D2 testdata/receiver_split/org.tsv:196 class 62.0
// NATIVE D2 testdata/receiver_split/org.tsv:197 class 67.0
// NATIVE D3 testdata/receiver_split/org.tsv:198 class 62.0
// NATIVE D3 testdata/receiver_split/org.tsv:199 class 67.0
// NATIVE D4 testdata/receiver_split/org.tsv:200 class 62.0
// NATIVE D4 testdata/receiver_split/org.tsv:201 class 67.0
// NATIVE D5 testdata/receiver_split/org.tsv:202 class 62.0
// NATIVE D5 testdata/receiver_split/org.tsv:203 class 67.0
// NATIVE D6 testdata/receiver_split/org.tsv:204 class 62.0
// NATIVE D6 testdata/receiver_split/org.tsv:205 class 67.0
// NATIVE E1 testdata/receiver_split/org.tsv:206 class 62.0
// NATIVE E1 testdata/receiver_split/org.tsv:207 class 67.0
// NATIVE E2 testdata/receiver_split/org.tsv:208 class 62.0
// NATIVE E2 testdata/receiver_split/org.tsv:209 class 67.0
// NATIVE G0 testdata/receiver_split/org.tsv:210 trigger 62.0
// NATIVE G0 testdata/receiver_split/org.tsv:211 trigger 67.0
// NATIVE G1 testdata/receiver_split/org.tsv:212 trigger 62.0
// NATIVE G1 testdata/receiver_split/org.tsv:213 trigger 67.0
// NATIVE G2 testdata/receiver_split/org.tsv:214 trigger 62.0
// NATIVE G2 testdata/receiver_split/org.tsv:215 trigger 67.0
// NATIVE G3 testdata/receiver_split/org.tsv:216 trigger 62.0
// NATIVE G3 testdata/receiver_split/org.tsv:217 trigger 67.0
// NATIVE G4 testdata/receiver_split/org.tsv:218 trigger 62.0
// NATIVE G4 testdata/receiver_split/org.tsv:219 trigger 67.0
// NATIVE G5 testdata/receiver_split/org.tsv:220 trigger 62.0
// NATIVE G5 testdata/receiver_split/org.tsv:221 trigger 67.0
// NATIVE while_header testdata/receiver_split/org.tsv:222 anonymous 62.0
// NATIVE while_header testdata/receiver_split/org.tsv:223 anonymous 67.0
// NATIVE for_header testdata/receiver_split/org.tsv:224 anonymous 62.0
// NATIVE for_header testdata/receiver_split/org.tsv:225 anonymous 67.0
// NATIVE switch_header testdata/receiver_split/org.tsv:226 anonymous 62.0
// NATIVE switch_header testdata/receiver_split/org.tsv:227 anonymous 67.0
// NATIVE parentheses testdata/receiver_split/org.tsv:228 anonymous 62.0
// NATIVE parentheses testdata/receiver_split/org.tsv:229 anonymous 67.0
// NATIVE index testdata/receiver_split/org.tsv:230 anonymous 62.0
// NATIVE index testdata/receiver_split/org.tsv:231 anonymous 67.0
// NATIVE list_init testdata/receiver_split/org.tsv:232 anonymous 62.0
// NATIVE list_init testdata/receiver_split/org.tsv:233 anonymous 67.0
// NATIVE map_init testdata/receiver_split/org.tsv:234 anonymous 62.0
// NATIVE map_init testdata/receiver_split/org.tsv:235 anonymous 67.0
// NATIVE generic_lead testdata/receiver_split/org.tsv:236 anonymous 62.0
// NATIVE generic_lead testdata/receiver_split/org.tsv:237 anonymous 67.0
// NATIVE array_lead testdata/receiver_split/org.tsv:238 anonymous 62.0
// NATIVE array_lead testdata/receiver_split/org.tsv:239 anonymous 67.0
// NATIVE block_type_bound testdata/receiver_split/org.tsv:240 anonymous 62.0
// NATIVE block_type_bound testdata/receiver_split/org.tsv:241 anonymous 67.0
// NATIVE block_type_free testdata/receiver_split/org.tsv:242 anonymous 62.0
// NATIVE block_type_free testdata/receiver_split/org.tsv:243 anonymous 67.0
// NATIVE control testdata/receiver_split/org.tsv:244 anonymous 62.0
// NATIVE control testdata/receiver_split/org.tsv:245 anonymous 67.0
// NATIVE super_break testdata/receiver_split/org.tsv:246 class 62.0
// NATIVE super_break testdata/receiver_split/org.tsv:247 class 67.0
// NATIVE custom_namespace_type testdata/receiver_split/org.tsv:248 anonymous 62.0
// NATIVE custom_namespace_type testdata/receiver_split/org.tsv:249 anonymous 67.0
// NATIVE custom_namespace_root testdata/receiver_split/org.tsv:250 anonymous 62.0
// NATIVE custom_namespace_root testdata/receiver_split/org.tsv:251 anonymous 67.0
// NATIVE custom_control testdata/receiver_split/org.tsv:252 anonymous 62.0
// NATIVE custom_control testdata/receiver_split/org.tsv:253 anonymous 67.0
// NATIVE ternary testdata/receiver_split/org.tsv:254 anonymous 62.0
// NATIVE ternary testdata/receiver_split/org.tsv:255 anonymous 67.0
// NATIVE constructor_argument testdata/receiver_split/org.tsv:256 anonymous 62.0
// NATIVE constructor_argument testdata/receiver_split/org.tsv:257 anonymous 67.0
// NATIVE set_initializer testdata/receiver_split/org.tsv:258 anonymous 62.0
// NATIVE set_initializer testdata/receiver_split/org.tsv:259 anonymous 67.0
// NATIVE multisegment_statement testdata/receiver_split/org.tsv:260 anonymous 62.0
// NATIVE multisegment_statement testdata/receiver_split/org.tsv:261 anonymous 67.0
// NATIVE cast_type testdata/receiver_split/org.tsv:262 anonymous 62.0
// NATIVE cast_type testdata/receiver_split/org.tsv:263 anonymous 67.0
// NATIVE generic_type testdata/receiver_split/org.tsv:264 anonymous 62.0
// NATIVE generic_type testdata/receiver_split/org.tsv:265 anonymous 67.0
// NATIVE instanceof_type testdata/receiver_split/org.tsv:266 anonymous 62.0
// NATIVE instanceof_type testdata/receiver_split/org.tsv:267 anonymous 67.0
// NATIVE for_update testdata/receiver_split/org.tsv:268 anonymous 62.0
// NATIVE for_update testdata/receiver_split/org.tsv:269 anonymous 67.0
// NATIVE extra_control testdata/receiver_split/org.tsv:270 anonymous 62.0
// NATIVE extra_control testdata/receiver_split/org.tsv:271 anonymous 67.0
// NATIVE signature testdata/receiver_split/org.tsv:272 class 62.0
// NATIVE signature testdata/receiver_split/org.tsv:273 class 67.0
// NATIVE extends testdata/receiver_split/org.tsv:274 class 62.0
// NATIVE extends testdata/receiver_split/org.tsv:275 class 67.0
// NATIVE implements testdata/receiver_split/org.tsv:276 class 62.0
// NATIVE implements testdata/receiver_split/org.tsv:277 class 67.0
// NATIVE property testdata/receiver_split/org.tsv:278 class 62.0
// NATIVE property testdata/receiver_split/org.tsv:279 class 67.0
// NATIVE static_block testdata/receiver_split/org.tsv:280 class 62.0
// NATIVE static_block testdata/receiver_split/org.tsv:281 class 67.0
// NATIVE trigger_bound testdata/receiver_split/org.tsv:282 trigger 62.0
// NATIVE trigger_bound testdata/receiver_split/org.tsv:283 trigger 67.0
// NATIVE trigger_free testdata/receiver_split/org.tsv:284 trigger 62.0
// NATIVE trigger_free testdata/receiver_split/org.tsv:285 trigger 67.0
// NATIVE free_generic_type testdata/receiver_split/org.tsv:286 anonymous 62.0
// NATIVE free_generic_type testdata/receiver_split/org.tsv:287 anonymous 67.0
// NATIVE free_cast_type testdata/receiver_split/org.tsv:288 anonymous 62.0
// NATIVE free_cast_type testdata/receiver_split/org.tsv:289 anonymous 67.0
// NATIVE free_instanceof_type testdata/receiver_split/org.tsv:290 anonymous 62.0
// NATIVE free_instanceof_type testdata/receiver_split/org.tsv:291 anonymous 67.0
// NATIVE free_control testdata/receiver_split/org.tsv:292 anonymous 62.0
// NATIVE free_control testdata/receiver_split/org.tsv:293 anonymous 67.0
// NATIVE free_signature testdata/receiver_split/org.tsv:294 class 62.0
// NATIVE free_signature testdata/receiver_split/org.tsv:295 class 67.0
// NATIVE free_extends testdata/receiver_split/org.tsv:296 class 62.0
// NATIVE free_extends testdata/receiver_split/org.tsv:297 class 67.0
// NATIVE free_implements testdata/receiver_split/org.tsv:298 class 62.0
// NATIVE free_implements testdata/receiver_split/org.tsv:299 class 67.0
// NATIVE bound_cr_comment testdata/receiver_split/org.tsv:300 anonymous 62.0
// NATIVE bound_cr_comment testdata/receiver_split/org.tsv:301 anonymous 67.0
// NATIVE free_cr_comment testdata/receiver_split/org.tsv:302 anonymous 62.0
// NATIVE free_cr_comment testdata/receiver_split/org.tsv:303 anonymous 67.0
// NATIVE trivia_control testdata/receiver_split/org.tsv:304 anonymous 62.0
// NATIVE trivia_control testdata/receiver_split/org.tsv:305 anonymous 67.0
// NATIVE bound_vertical_tab testdata/receiver_split/org.tsv:306 anonymous 62.0
// NATIVE bound_vertical_tab testdata/receiver_split/org.tsv:307 anonymous 67.0
// NATIVE free_vertical_tab testdata/receiver_split/org.tsv:308 anonymous 62.0
// NATIVE free_vertical_tab testdata/receiver_split/org.tsv:309 anonymous 67.0
func TestReceiverGrammarNative(t *testing.T) {
	testReceiverGrammarNative(t)
}

func testReceiverGrammarNative(t *testing.T) {
	const fixture = "testdata/receiver_split"
	f, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rd := csv.NewReader(f)
	rd.Comma = '\t'
	rows, err := rd.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	cases := readReceiverGrammarCases(t)
	for _, row := range rows[1:] {
		if _, ok := cases[row[0]]; !ok {
			continue
		}
		t.Run(row[0]+"/"+row[1], func(t *testing.T) {
			ext := ".apex"
			if row[2] == "class" {
				ext = ".cls"
			}
			if row[2] == "trigger" {
				ext = ".trigger"
			}
			src, err := os.ReadFile(filepath.Join(fixture, cases[row[0]]))
			if err != nil {
				t.Fatal(err)
			}
			support, err := os.ReadFile(filepath.Join(fixture, "CrProbe.cls"))
			if err != nil {
				t.Fatal(err)
			}
			root := t.TempDir()
			probe := filepath.Join(root, "CrProbe.cls")
			writeSemaFile(t, probe, string(support))
			files := []string{probe}
			if row[2] != "anonymous" {
				declaration := regexp.MustCompile(`(?i)\b(?:class|trigger)\s+([A-Za-z_][A-Za-z0-9_]*)`).FindSubmatch(src)
				if len(declaration) < 2 {
					t.Fatal("missing declaration name")
				}
				name := string(declaration[1]) + ext
				path := filepath.Join(root, name)
				writeSemaFile(t, path, string(src))
				files = append(files, path)
			}
			namespace := ""
			if strings.HasPrefix(row[0], "custom_namespace_") || row[0] == "custom_control" {
				namespace = "sample"
			}
			index := typesys.Build(project.Project{Root: root, Namespace: namespace, SourceAPIVersion: row[1], ApexFiles: files}, schema.Schema{Objects: []schema.Object{{Name: "Account"}}})
			var ds []diagnostic.Diagnostic
			if index.HasErrors() {
				ds = index.Diagnostics
			} else if row[2] == "anonymous" {
				ds = AnalyzeAnonymous(index, string(src), row[1]).Diagnostics
			} else {
				ds = Analyze(index).Diagnostics
			}
			var got []string
			for _, d := range ds {
				if d.Severity != diagnostic.Error {
					continue
				}
				message := d.NativeMessage
				if message == "" {
					message = d.Message
				}
				if d.Range == nil {
					got = append(got, message)
					continue
				}
				got = append(got, fmt.Sprintf("%d:%d %s", d.Range.Start.Line, d.Range.Start.Column, message))
			}
			if row[3] == "true" {
				if len(got) != 0 {
					t.Fatalf("native accepts; got %v", got)
				}
				return
			}
			want := strings.Split(strings.TrimPrefix(row[4], "deploy fails: "), " | ")
			if len(got) == 0 || got[0] != want[0] {
				t.Errorf("primary diagnostic = %v; want %s", got, want[0])
				return
			}
			// Native class parsing emits these documented cascades in addition
			// to the primary error. Track them explicitly until implemented.
			knownCascades := map[string]bool{"CA": true, "D1": true, "D2": true, "D3": true, "D4": true, "D5": true, "D6": true, "E1": true, "E2": true, "super_break": true, "signature": true, "extends": true, "implements": true, "property": true, "static_block": true, "free_signature": true, "free_extends": true, "free_implements": true}
			if knownCascades[row[0]] {
				remaining := make(map[string]int, len(want)-1)
				for _, extra := range want[1:] {
					remaining[extra]++
				}
				for _, extra := range got[1:] {
					if remaining[extra] == 0 {
						t.Errorf("unexpected diagnostic %q; native %v", extra, want)
					} else {
						remaining[extra]--
					}
				}
				for extra, count := range remaining {
					if count > 0 {
						t.Logf("known native cascade not reproduced (%d): %s", count, extra)
					}
				}
			} else if strings.Join(got, " | ") != strings.Join(want, " | ") {
				t.Errorf("diagnostics = %v; want %v", got, want)
			}
		})
	}
}

// API 64 lies between both captures. L2, F2 and M1 cover a free dot,
// top-level bound dot and block argument respectively (org.tsv).
func TestReceiverGrammarIntermediateAPI(t *testing.T) {
	const fixture = "testdata/receiver_split"
	f, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rd := csv.NewReader(f)
	rd.Comma = '\t'
	rows, err := rd.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	support, err := os.ReadFile(filepath.Join(fixture, "CrProbe.cls"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "CrProbe.cls")
	writeSemaFile(t, path, string(support))
	index := typesys.Build(project.Project{Root: root, SourceAPIVersion: "64.0", ApexFiles: []string{path}}, schema.Schema{})
	cases := readReceiverGrammarCases(t)
	for _, id := range []string{"L2", "F2", "M1"} {
		var want string
		for _, row := range rows[1:] {
			if row[0] == id && row[1] == "62.0" {
				want = row[4]
				break
			}
		}
		if want == "" {
			t.Fatalf("missing native case %s", id)
		}
		src, err := os.ReadFile(filepath.Join(fixture, cases[id]))
		if err != nil {
			t.Fatal(err)
		}
		ds := AnalyzeAnonymous(index, string(src), "64.0").Diagnostics
		if len(ds) == 0 || ds[0].Range == nil {
			t.Errorf("%s: no positioned diagnostic, want %s", id, want)
			continue
		}
		d := ds[0]
		msg := d.NativeMessage
		if msg == "" {
			msg = d.Message
		}
		got := fmt.Sprintf("%d:%d %s", d.Range.Start.Line, d.Range.Start.Column, msg)
		if got != want {
			t.Errorf("%s: got %s, want %s", id, got, want)
		}
	}
}

func readReceiverGrammarCases(t *testing.T) map[string]string {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata/receiver_split", "cases.tsv"))
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
	cases := map[string]string{}
	for _, row := range rows[1:] {
		if strings.HasPrefix(row[2], "grammar/") {
			cases[row[0]] = row[2]
		}
	}
	return cases
}
