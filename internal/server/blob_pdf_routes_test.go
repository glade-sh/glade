package server

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

func TestVisualforcePDFLocalAndConfiguredHTTPRouteTwins(t *testing.T) {
	for _, external := range []bool{false, true} {
		for _, renderAs := range []bool{false, true} {
			t.Run(fmt.Sprintf("external=%v/renderAs=%v", external, renderAs), func(t *testing.T) {
				org := testOrg()
				source := testSourceMetadata(t)
				root := source.Project.Root
				if external {
					command := filepath.Join(root, "render-pdf")
					if err := os.WriteFile(command, []byte("#!/bin/sh\ncat >/dev/null\nprintf '%%PDF-1.4 external renderer'\n"), 0700); err != nil {
						t.Fatal(err)
					}
					writeServerTestFile(t, filepath.Join(root, "glade.yml"), "project:\n  root: .\npdf:\n  command: '"+command+"'\n")
				}
				attr := ""
				if renderAs {
					attr = ` renderAs="pdf"`
				}
				writeServerTestFile(t, filepath.Join(root, "force-app/main/default/pages/Invoice.page"), `<apex:page`+attr+`><p>one</p><div style="page-break-before:always"></div><p>two</p><select><option>United States</option><option>Canada</option></select></apex:page>`)
				reloaded, err := project.Load(root)
				if err != nil {
					t.Fatal(err)
				}
				source, err = NewSourceMetadataFromProject(reloaded)
				if err != nil {
					t.Fatal(err)
				}
				handler := newVisualforceHTMLTestServer(t, &org, source)
				url := "/apex/Invoice"
				if !renderAs {
					url += "?renderAs=pdf"
				}
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, url, nil))
				if rec.Code != http.StatusOK {
					t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
				}
				if !external {
					var text strings.Builder
					data := rec.Body.Bytes()
					for _, m := range regexp.MustCompile(`/Length (\d+)>>stream\n`).FindAllSubmatchIndex(data, -1) {
						n, _ := strconv.Atoi(string(data[m[2]:m[3]]))
						if n > len(data)-m[1] {
							t.Fatal("invalid stream length")
						}
						r, err := zlib.NewReader(bytes.NewReader(data[m[1] : m[1]+n]))
						if err != nil {
							t.Fatal(err)
						}
						content, err := io.ReadAll(r)
						r.Close()
						if err != nil {
							t.Fatal(err)
						}
						text.Write(content)
					}
					if !strings.Contains(text.String(), "(United States Canada)Tj") {
						t.Fatalf("HTTP PDF option text=%q", text.String())
					}
				}
				data := rec.Body.String()
				if external {
					if data != "%PDF-1.4 external renderer" {
						t.Fatal("configured renderer was not used")
					}
				} else if !strings.Contains(data, "/Type/Pages/Count 2") || !strings.Contains(data, "/BaseFont/Times-Roman") || !strings.Contains(data, "/Filter/FlateDecode") {
					t.Fatal("missing shared PDF layout")
				}
			})
		}
	}
}
