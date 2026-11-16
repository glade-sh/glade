package vm

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
)

// Shift-JIS is the JIS X 0208 encoding and Windows-31J is code page 932. The
// native rows encode single characters and decode escaped byte sequences under
// each name and spelling.
// NATIVE EncodingUtil.urlEncode urlDecode Shift-JIS Windows-31J shift_jis_windows_31j/org.tsv anonymous 62.0 67.0
func TestURLCharsetJapaneseNativeConformance(t *testing.T) {
	file, err := os.Open("../apextest/testdata/conformance/shift_jis_windows_31j/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil || len(rows) < 200 || strings.Join(rows[0], ",") != "api,op,charset,input,expected" {
		t.Fatalf("incomplete native table: rows=%d err=%v", len(rows), err)
	}
	for _, row := range rows[1:] {
		api, op, charset, input, expected := row[0], row[1], row[2], row[3], row[4]
		t.Run(api+"/"+op+"/"+charset+"/"+input, func(t *testing.T) {
			switch op {
			case "encode":
				code, err := strconv.Atoi(input)
				if err != nil {
					t.Fatal(err)
				}
				got, err := urlEncodeWithCharset("", string(rune(code)), charset)
				if err != nil || got != expected {
					t.Fatalf("native %q; Glade %q err=%v", expected, got, err)
				}
			case "decode":
				decoded, err := urlDecodeWithCharset("", input, charset)
				if err != nil {
					t.Fatalf("native %q; Glade err=%v", expected, err)
				}
				var points []string
				for _, unit := range utf16Units(decoded) {
					points = append(points, strconv.Itoa(int(unit)))
				}
				if got := strings.Join(points, "/"); got != expected {
					t.Fatalf("native %q; Glade %q", expected, got)
				}
			default:
				t.Fatal(fmt.Sprintf("unknown op %q", op))
			}
		})
	}
}

// Every accepted spelling must reach the same codec as its canonical name.
// NATIVE spellings SJIS x-sjis csShiftJIS MS932 windows-31j apex_release_gates/org.tsv SJ rows anonymous 62.0 67.0
func TestURLCharsetJapaneseSpellings(t *testing.T) {
	for _, name := range []string{"Shift-JIS", "Shift_JIS", "SJIS", "x-sjis", "csShiftJIS"} {
		got, err := urlEncodeWithCharset("", string(rune(9312)), name)
		if err != nil || got != "%3F" {
			t.Errorf("%s: circled digit one encodes %q err=%v, native %%3F", name, got, err)
		}
	}
	for _, name := range []string{"Windows-31J", "windows-31j", "MS932"} {
		got, err := urlEncodeWithCharset("", string(rune(9312)), name)
		if err != nil || got != "%87%40" {
			t.Errorf("%s: circled digit one encodes %q err=%v, native %%87%%40", name, got, err)
		}
	}
}

func utf16Units(text string) []uint16 {
	var out []uint16
	for _, r := range text {
		if r >= 0x10000 {
			r -= 0x10000
			out = append(out, uint16(0xD800+(r>>10)), uint16(0xDC00+(r&0x3FF)))
			continue
		}
		out = append(out, uint16(r))
	}
	return out
}
