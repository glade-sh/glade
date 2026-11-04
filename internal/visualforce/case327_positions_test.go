package visualforce

import (
	"fmt"
	"strings"
	"testing"
)

func TestCase327SourceCoordinates(t *testing.T) {
	cases := []string{
		"<apex:page>é\r\n<apex:outputText value=\"x\ny\"/><b>z</b></apex:page>",
		"\xff<script>var x = '<notatag>';</script>\n<style>.x{content:'<x>'}</style><br/>",
		"<table><tr><td>🐝</td></tr></table><apex:outputText value='ok'/>",
	}
	for _, source := range cases {
		tags := scanSourceTags(source)
		from := 0
		for _, tag := range tags {
			rel := strings.Index(source[from:], "<"+tag.name)
			if rel < 0 {
				t.Fatalf("tag %q not found after %d", tag.name, from)
			}
			off := from + rel
			line, col := lineColumnAt(source, off)
			endLine, endCol := lineColumnAt(source, off+1+len(tag.name))
			if tag.line != line || tag.column != col || tag.nameEndLine != endLine || tag.nameEndColumn != endCol {
				t.Fatalf("%q got %d:%d-%d:%d want %d:%d-%d:%d", tag.name, tag.line, tag.column, tag.nameEndLine, tag.nameEndColumn, line, col, endLine, endCol)
			}
			from = off + 1 + len(tag.name)
		}
	}
}

func BenchmarkCase327SourceCoordinates(b *testing.B) {
	for _, count := range []int{100, 1000, 5000} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			source := strings.Repeat("é\n<apex:outputText value='ok'/>", count)
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				scanSourceTags(source)
			}
		})
	}
}
