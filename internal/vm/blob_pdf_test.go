package vm

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

func blobPDFThroughRoute(t *testing.T, input, route string, now time.Time) []byte {
	t.Helper()
	call := "Blob.toPdf"
	if route == "instance" {
		call = "Blob.valueOf('receiver').toPdf"
	}
	quoted := "'" + strings.NewReplacer("\\", "\\\\", "'", "\\'", "\n", "\\n").Replace(input) + "'"
	program, err := CompileAnonymous("Blob pdf = " + call + "(" + quoted + ");")
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.fakeNow = now
	result, err := machine.Execute(program)
	if err != nil {
		t.Fatal(err)
	}
	return []byte(result.Vars["pdf"].Fields["value"].Text)
}

// parseBlobPDF checks classic xref offsets and direct stream lengths, then
// extracts escaped literal strings from Flate content streams.
func parseBlobPDF(t *testing.T, data []byte) (int, string) {
	t.Helper()
	sx := regexp.MustCompile(`startxref\s+(\d+)`).FindSubmatch(data)
	if sx == nil {
		t.Fatal("missing startxref")
	}
	offset, _ := strconv.Atoi(string(sx[1]))
	if offset < 0 || offset >= len(data) {
		t.Fatal("invalid xref offset")
	}
	lines := strings.Split(string(data[offset:]), "\n")
	if lines[0] != "xref" {
		t.Fatal("xref offset does not address xref")
	}
	var start, count int
	if _, err := fmt.Sscanf(lines[1], "%d %d", &start, &count); err != nil || start != 0 || count < 6 || len(lines) < count+2 {
		t.Fatal("invalid xref table")
	}
	free := map[int]int{}
	for n, line := range lines[2 : count+2] {
		var off, generation int
		var state string
		if _, err := fmt.Sscanf(line, "%d %d %s", &off, &generation, &state); err != nil {
			t.Fatal(err)
		}
		if state == "f" {
			free[n] = off
		}
		if state == "n" && (off < 0 || off >= len(data) || !bytes.HasPrefix(data[off:], []byte(fmt.Sprintf("%d 0 obj", n)))) {
			t.Fatalf("invalid object %d offset %d", n, off)
		}
	}
	linked := map[int]bool{0: true}
	for next := free[0]; next != 0; next = free[next] {
		if _, ok := free[next]; !ok || linked[next] {
			t.Fatal("invalid free-object chain")
		}
		linked[next] = true
	}
	if len(linked) != len(free) {
		t.Fatal("unlinked free xref entry")
	}
	var text []string
	streams := regexp.MustCompile(`/Length (\d+)>>stream\n`).FindAllSubmatchIndex(data, -1)
	for _, m := range streams {
		length, _ := strconv.Atoi(string(data[m[2]:m[3]]))
		end := m[1] + length
		if end > len(data) || !bytes.HasPrefix(data[end:], []byte("\nendstream")) {
			t.Fatal("invalid direct stream length")
		}
		zr, err := zlib.NewReader(bytes.NewReader(data[m[1]:end]))
		if err != nil {
			t.Fatal(err)
		}
		content, err := io.ReadAll(zr)
		zr.Close()
		if err != nil {
			t.Fatal(err)
		}
		for _, lit := range regexp.MustCompile(`\(((?:\\.|[^\\)])*)\)Tj`).FindAllSubmatch(content, -1) {
			text = append(text, regexp.MustCompile(`\\(.)`).ReplaceAllString(string(lit[1]), "$1"))
		}
	}
	if len(streams) == 0 {
		t.Fatal("missing Flate streams")
	}
	pages := len(regexp.MustCompile(`/Type/Page(?:/|\s)`).FindAll(data, -1))
	return pages, strings.Join(text, " ")
}

func TestBlobPDFStaticAndInstanceStructureRoundTrip(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	for _, route := range []string{"static", "instance"} {
		t.Run(route, func(t *testing.T) {
			input := `<style>.pb { page-break-before: always; }</style><p>one (a)\b café 日本語</p><div class="pb"></div><p>two</p>`
			data := blobPDFThroughRoute(t, input, route, now)
			pages, text := parseBlobPDF(t, data)
			if pages != 2 || text != "one (a)\\b caf\xe9 two" {
				t.Fatalf("pages=%d text=%q", pages, text)
			}
			for _, want := range []string{"%PDF-1.4\n%\xe2\xe3\xcf\xd3\n", "/Type/Pages/Count 2", "/MediaBox[0 0 612 792]", "/BaseFont/Times-Roman", "/Encoding/WinAnsiEncoding", "/Filter/FlateDecode", "/Producer(glade ", "/CreationDate(D:20260102030405Z)", "/ID [<"} {
				if !bytes.Contains(data, []byte(want)) {
					t.Errorf("missing %q", want)
				}
			}
			if !bytes.Equal(data, blobPDFThroughRoute(t, input, route, now)) {
				t.Error("same clock/input must be deterministic")
			}
			if bytes.Equal(data, blobPDFThroughRoute(t, input, route, now.Add(time.Second))) {
				t.Error("dates must follow the VM clock")
			}
		})
	}
}

func TestBlobPDFStaticAndInstancePageLayout(t *testing.T) {
	for _, route := range []string{"static", "instance"} {
		for _, tc := range []struct {
			name, input, box string
			pages            int
		}{
			{"empty", "", "612 792", 1},
			{"landscape", `<style>@page{size:landscape}</style><p>wide</p>`, "792 612", 1},
			{"a4", `<style>@page{size:A4}</style>a4`, "595.28 841.88", 1},
			{"inlineBreak", `one<div style="page-break-before:always">two</div>`, "612 792", 2},
			{"paragraphOverflow", strings.Repeat("<p>line</p>", 150), "612 792", 3},
			{"unbrokenMegabyte", strings.Repeat("x", 1000000), "612 792", 1},
		} {
			t.Run(route+"/"+tc.name, func(t *testing.T) {
				data := blobPDFThroughRoute(t, tc.input, route, time.Time{})
				pages, _ := parseBlobPDF(t, data)
				if pages != tc.pages || !bytes.Contains(data, []byte("/MediaBox[0 0 "+tc.box+"]")) {
					t.Fatalf("pages=%d want=%d; box=%s", pages, tc.pages, tc.box)
				}
			})
		}
	}
}

func TestBlobPDFNativeSamplesStructureRoundTrip(t *testing.T) {
	for _, name := range []string{"sample62.0_SAMPLE_PLAIN.pdf", "sample62.0_TEST.pdf"} {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile("../apextest/testdata/conformance/blob_pdf_rendering/" + name)
			if err != nil {
				t.Fatal(err)
			}
			pages, text := parseBlobPDF(t, data)
			if pages != 1 || text != "Hello PDF world" {
				t.Fatalf("pages=%d text=%q", pages, text)
			}
		})
	}
}

func TestBlobPDFStaticAndInstanceWinAnsiWhitespace(t *testing.T) {
	for _, route := range []string{"static", "instance"} {
		for _, input := range []string{"Â… text", "text Â…", "text Â… text"} {
			t.Run(route+"/"+input, func(t *testing.T) {
				data := blobPDFThroughRoute(t, input, route, time.Time{})
				_, text := parseBlobPDF(t, data)
				want := strings.ReplaceAll(input, "Â…", "\xc2\x85")
				if text != want {
					t.Fatalf("WinAnsi text=%q want=%q", text, want)
				}
			})
		}
	}
}

func TestBlobPDFStaticAndInstanceOptionSpacing(t *testing.T) {
	for _, route := range []string{"static", "instance"} {
		for _, tc := range []struct{ name, input, want string }{
			{"inlineText", `<p>Can<span>a</span>da</p>`, "Canada"},
			{"adjacentOptions", `<select><option>United States</option><option>Canada</option></select>`, "United States Canada"},
			{"surroundingText", `Before<select><option>United States</option><option>Canada</option></select>After`, "Before United States Canada After"},
			{"existingSpaces", `Before <select><option> United States </option><option> Canada </option></select> After`, "Before United States Canada After"},
		} {
			t.Run(route+"/"+tc.name, func(t *testing.T) {
				data := blobPDFThroughRoute(t, tc.input, route, time.Time{})
				_, text := parseBlobPDF(t, data)
				if text != tc.want {
					t.Fatalf("option text=%q want=%q", text, tc.want)
				}
			})
		}
	}
}
