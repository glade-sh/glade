package vm

import (
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"golang.org/x/text/unicode/runenames"
)

// java.lang.Character.codePointOf resolves \N{name}. The name is trimmed of
// characters up to U+0020 and upper-cased. It is looked up in the Unicode name
// table; failing that, the text after the last space must be a hexadecimal
// code point whose Character.getName is the whole text. getName gives the
// table name, or "<BLOCK NAME> <hex>" for an assigned character that has none
// (native rows d20_*, d21_*).

var (
	javaNamesOnce  sync.Once
	javaNameToRune map[string]rune
)

// javaControlNames are the Unicode 1.0 names Java's name table holds for the
// control characters. The Unicode name table gives them only as <control>.
var javaControlNames = map[rune]string{
	0x00: "NULL", 0x01: "START OF HEADING", 0x02: "START OF TEXT", 0x03: "END OF TEXT", 0x04: "END OF TRANSMISSION",
	0x05: "ENQUIRY", 0x06: "ACKNOWLEDGE", 0x07: "BELL", 0x08: "BACKSPACE", 0x09: "CHARACTER TABULATION",
	0x0A: "LINE FEED (LF)", 0x0B: "LINE TABULATION", 0x0C: "FORM FEED (FF)", 0x0D: "CARRIAGE RETURN (CR)",
	0x0E: "SHIFT OUT", 0x0F: "SHIFT IN", 0x10: "DATA LINK ESCAPE", 0x11: "DEVICE CONTROL ONE", 0x12: "DEVICE CONTROL TWO",
	0x13: "DEVICE CONTROL THREE", 0x14: "DEVICE CONTROL FOUR", 0x15: "NEGATIVE ACKNOWLEDGE", 0x16: "SYNCHRONOUS IDLE",
	0x17: "END OF TRANSMISSION BLOCK", 0x18: "CANCEL", 0x19: "END OF MEDIUM", 0x1A: "SUBSTITUTE", 0x1B: "ESCAPE",
	0x1C: "INFORMATION SEPARATOR FOUR", 0x1D: "INFORMATION SEPARATOR THREE", 0x1E: "INFORMATION SEPARATOR TWO",
	0x1F: "INFORMATION SEPARATOR ONE", 0x7F: "DELETE",
	0x85: "NEXT LINE (NEL)", 0x86: "START OF SELECTED AREA", 0x87: "END OF SELECTED AREA", 0x88: "CHARACTER TABULATION SET",
	0x89: "CHARACTER TABULATION WITH JUSTIFICATION", 0x8A: "LINE TABULATION SET", 0x8B: "PARTIAL LINE FORWARD",
	0x8C: "PARTIAL LINE BACKWARD", 0x8D: "REVERSE LINE FEED", 0x8E: "SINGLE SHIFT TWO", 0x8F: "SINGLE SHIFT THREE",
	0x90: "DEVICE CONTROL STRING", 0x91: "PRIVATE USE ONE", 0x92: "PRIVATE USE TWO", 0x93: "SET TRANSMIT STATE",
	0x94: "CANCEL CHARACTER", 0x95: "MESSAGE WAITING", 0x96: "START OF GUARDED AREA", 0x97: "END OF GUARDED AREA",
	0x98: "START OF STRING", 0x9A: "SINGLE CHARACTER INTRODUCER", 0x9B: "CONTROL SEQUENCE INTRODUCER",
	0x9C: "STRING TERMINATOR", 0x9D: "OPERATING SYSTEM COMMAND", 0x9E: "PRIVACY MESSAGE", 0x9F: "APPLICATION PROGRAM COMMAND",
}

// javaNamelessBlocks are the java.lang.Character.UnicodeBlock names of the
// assigned characters that carry no name of their own (ideographs, Hangul
// syllables, private use, surrogates, unnamed controls).
var javaNamelessBlocks = []struct {
	lo, hi rune
	name   string
}{
	{0x3400, 0x4DBF, "CJK UNIFIED IDEOGRAPHS EXTENSION A"},
	{0x4E00, 0x9FFF, "CJK UNIFIED IDEOGRAPHS"},
	{0xAC00, 0xD7AF, "HANGUL SYLLABLES"},
	{0xD800, 0xDB7F, "HIGH SURROGATES"},
	{0xDB80, 0xDBFF, "HIGH PRIVATE USE SURROGATES"},
	{0xDC00, 0xDFFF, "LOW SURROGATES"},
	{0xE000, 0xF8FF, "PRIVATE USE AREA"},
	{0x17000, 0x187FF, "TANGUT"},
	{0x18800, 0x18AFF, "TANGUT COMPONENTS"},
	{0x18B00, 0x18CFF, "KHITAN SMALL SCRIPT"},
	{0x18D00, 0x18D8F, "TANGUT SUPPLEMENT"},
	{0x1B170, 0x1B2FF, "NUSHU"},
	{0x20000, 0x2A6DF, "CJK UNIFIED IDEOGRAPHS EXTENSION B"},
	{0x2A700, 0x2B73F, "CJK UNIFIED IDEOGRAPHS EXTENSION C"},
	{0x2B740, 0x2B81F, "CJK UNIFIED IDEOGRAPHS EXTENSION D"},
	{0x2B820, 0x2CEAF, "CJK UNIFIED IDEOGRAPHS EXTENSION E"},
	{0x2CEB0, 0x2EBEF, "CJK UNIFIED IDEOGRAPHS EXTENSION F"},
	{0x30000, 0x3134F, "CJK UNIFIED IDEOGRAPHS EXTENSION G"},
	{0x31350, 0x323AF, "CJK UNIFIED IDEOGRAPHS EXTENSION H"},
	{0xF0000, 0xFFFFF, "SUPPLEMENTARY PRIVATE USE AREA A"},
	{0x100000, 0x10FFFF, "SUPPLEMENTARY PRIVATE USE AREA B"},
}

func javaInitNames() {
	javaNamesOnce.Do(func() {
		javaNameToRune = make(map[string]rune, 41000)
		for r := rune(0); r <= utf8.MaxRune; r++ {
			if name := runenames.Name(r); name != "" && name[0] != '<' {
				javaNameToRune[name] = r
			}
		}
		for r, name := range javaControlNames {
			javaNameToRune[name] = r
		}
	})
}

// javaCharacterName mirrors Character.getName for the characters Glade can
// name: the table name, else the block form for an assigned nameless
// character, else "".
func javaCharacterName(r rune) string {
	if name, ok := javaControlNames[r]; ok {
		return name
	}
	if name := runenames.Name(r); name != "" && name[0] != '<' {
		return name
	}
	if runenames.Name(r) == "" {
		return ""
	}
	for _, block := range javaNamelessBlocks {
		if r >= block.lo && r <= block.hi {
			return block.name + " " + strings.ToUpper(strconv.FormatInt(int64(r), 16))
		}
	}
	return ""
}

// javaCharacterByName is Character.codePointOf.
func javaCharacterByName(name string) (rune, bool) {
	name = strings.ToUpper(strings.TrimFunc(name, func(r rune) bool { return r <= ' ' }))
	javaInitNames()
	if r, ok := javaNameToRune[name]; ok {
		return r, true
	}
	if off := strings.LastIndexByte(name, ' '); off >= 0 {
		if v, err := strconv.ParseInt(name[off+1:], 16, 32); err == nil && v >= 0 && v <= utf8.MaxRune {
			if javaCharacterName(rune(v)) == name {
				return rune(v), true
			}
		}
	}
	return 0, false
}
