package sema

import (
	"strings"
	"testing"
)

func TestQueryKeywordCommentsNativeRoutes(t *testing.T) {
	runCapturedSemanticBodyAnalysis(t, "testdata/query_keyword_comments")
}

func TestQueryKeywordCommentSpansNativeRows(t *testing.T) {
	cases, rows := readCapturedBodyRows(t, "testdata/query_keyword_comments")
	for _, row := range rows {
		t.Run(strings.Join(row[:3], "/"), func(t *testing.T) {
			body := cases[row[2]].Body
			pos := strings.Index(body, "SELECT")
			if pos < 0 {
				pos = strings.Index(body, "FIND '")
			}
			query := pos >= 0
			if !query {
				pos = strings.Index(body, "[0]") + 1
			}
			ignored := newSemaIgnoredText(body)
			if got := ignored.inSOQLLiteral(pos); got != query {
				t.Fatalf("native query=%t; span membership=%t at byte %d", query, got, pos)
			}
			if got := ignored.contains(pos); got != query {
				t.Fatalf("native query=%t; declaration exclusion=%t at byte %d", query, got, pos)
			}
		})
	}
}

func TestQueryKeywordCommentExtractionNativeRows(t *testing.T) {
	cases, rows := readCapturedBodyRows(t, "testdata/query_keyword_comments")
	for _, row := range rows {
		if row[2] == "index_boundary" {
			continue
		}
		t.Run(strings.Join(row[:3], "/"), func(t *testing.T) {
			body := cases[row[2]].Body
			keyword := "SELECT"
			offset := strings.Index(body, keyword)
			if offset < 0 {
				keyword = "FIND"
				offset = strings.Index(body, keyword)
			}
			spans := newSemaCodeSpans(body)
			literals := append(semaSOQLLiterals(body, spans), semaSOSLLiterals(body, spans)...)
			if len(literals) != 1 {
				t.Fatalf("native query not extracted: %+v", literals)
			}
			if literals[0].queryOffset != offset || !strings.HasPrefix(literals[0].text, keyword) {
				t.Fatalf("query source mapping: %+v; keyword offset %d", literals[0], offset)
			}
		})
	}
}
