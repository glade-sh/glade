package visualforce

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type presentationOutcome struct {
	Page        Page
	PageError   string
	Tree        *MarkupNode
	TreeError   string
	HTML        string
	RenderError string
	Diagnostic  string
}

func TestPresentationValidationOutcomeIdentity(t *testing.T) {
	cases := map[string]string{
		"ordinary":       "<apex:page>\r\n<apex:outputText value='hello'/>é<b>text</b></apex:page>",
		"malformed_utf8": "<apex:page>\xff\r\n<apex:outputText value='x'/></apex:page>",
		"diagnostic":     "<apex:page>é\r\n<apex:outputText value='x' unexpected='y'/></apex:page>",
		"duplicate":      "<apex:page>\r\n<apex:outputText value='x' value='y'/></apex:page>",
	}
	for name, source := range cases {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "Example.page")
			if err := os.WriteFile(path, []byte(source), 0644); err != nil {
				t.Fatal(err)
			}
			var out presentationOutcome
			page, err := parsePageFile(path, "67.0")
			if err != nil {
				out.PageError = err.Error()
			} else {
				page.File = filepath.Base(page.File)
				out.Page = page
			}
			out.Tree, err = ParseMarkupTree(source)
			if err != nil {
				out.TreeError = err.Error()
			} else {
				html, renderErr := RenderMarkupTree(out.Tree, &RenderContext{PageName: "Example", Expression: &ExpressionContext{}})
				out.HTML = fmt.Sprintf("%x", sha256.Sum256([]byte(html)))
				err = renderErr
				if err != nil {
					out.RenderError = err.Error()
				}
			}
			if err = validatePresentationSource(source, "Example"); err != nil {
				out.Diagnostic = err.Error()
			}
			got, err := json.MarshalIndent(out, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			got = append(got, '\n')
			golden := filepath.Join("testdata", "presentation-outcomes", name+".json")
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != string(want) {
				t.Fatalf("outcome changed\ngot %s\nwant %s", got, want)
			}
		})
	}
}

func BenchmarkPresentationValidation4000Tags(b *testing.B) {
	source := "<apex:page>" + strings.Repeat("é\r\n<apex:outputText value='ok'/>", 4000) + "</apex:page>"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := validatePresentationSource(source, "Example"); err != nil {
			b.Fatal(err)
		}
	}
}
