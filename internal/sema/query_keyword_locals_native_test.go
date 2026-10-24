package sema

import (
	"os"
	"strings"
	"testing"
)

func TestQueryKeywordLocalsNativeRoutes(t *testing.T) {
	runCapturedSemanticBodyAnalysis(t, "testdata/query_keyword_locals")
}

func TestQueryKeywordLocalScanNativeRows(t *testing.T) {
	cases, rows := readCapturedBodyRows(t, "testdata/query_keyword_locals")
	for _, row := range rows {
		if row[3] != "accepted" || !strings.HasSuffix(row[2], "_local") {
			continue
		}
		t.Run(strings.Join(row[:3], "/"), func(t *testing.T) {
			body := cases[row[2]].Body
			matches := findSemaLocalDeclMatches(body)
			found := false
			for _, match := range matches {
				if body[match[4]:match[5]] != "value" {
					continue
				}
				found = true
				if semaLocalDeclMatchInIgnoredText(newSemaIgnoredText(body), match) {
					t.Fatalf("native local incorrectly ignored: %s", body[match[2]:match[5]])
				}
			}
			if !found {
				t.Fatal("captured local missing from declaration scan")
			}
		})
	}
}

func TestQueryKeywordMultipleLocalScopeNativeRows(t *testing.T) {
	cases, rows := readCapturedBodyRows(t, "testdata/query_keyword_locals")
	for _, row := range rows {
		if row[2] != "multiple_locals" || row[1] == "anonymous" {
			continue
		}
		t.Run(strings.Join(row[:3], "/"), func(t *testing.T) {
			idx := capturedBodyIndex(t, "testdata/query_keyword_locals", row[0], row[1], cases[row[2]].Body)
			model := buildSemaTypeMemberView(idx)
			a := NewAnalyzer()
			a.prepareAnalysisModel(idx)
			checked := false
			for _, typ := range idx.Types {
				if typ.Name != "GardenBodyProbe" {
					continue
				}
				raw, err := os.ReadFile(typ.File)
				if err != nil {
					t.Fatal(err)
				}
				for _, member := range typ.Members {
					if member.Name != "observe" {
						continue
					}
					checked = true
					body, offset, ok := semaBodyFromRange(string(raw), member.BodyRange)
					if !ok {
						t.Fatal("missing captured body")
					}
					scopes, _ := a.collectBodyScopes(typ, member, body, offset, string(raw), map[string]string{semaCurrentTypeScopeKey: typ.Name}, model)
					for _, name := range []string{"first", "second"} {
						if got, ok := scopes.visibleAt(name, strings.Index(body, "System.assertEquals")); !ok || got != "Find" {
							t.Fatalf("native local %s missing from scope: %s/%t", name, got, ok)
						}
					}
				}
			}
			if !checked {
				t.Fatal("missing captured method")
			}
		})
	}
}
