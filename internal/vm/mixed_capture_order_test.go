package vm

import (
	"fmt"
	"strings"
	"testing"
)

// Native API 62/67/68 anonymous and @IsTest observations are exported in
// internal/apextest/testdata/conformance/value_capture_native.tsv.

func TestMixedCaptureGroupsFollowOpeningOrder(t *testing.T) {
	type capture struct {
		value      any
		start, end int
	}
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, tc := range []struct {
			name, pattern, input string
			groups               []capture
		}{
			{"namedFirst", `^(?<head>a)(b)$`, "ab", []capture{{"ab", 0, 2}, {"a", 0, 1}, {"b", 1, 2}}},
			{"namedLast", `^(a)(?<tail>b)$`, "ab", []capture{{"ab", 0, 2}, {"a", 0, 1}, {"b", 1, 2}}},
			{"optionalPresent", `^(?<stem>[0-9]+)(\.([0-9]+))?$`, "7.8", []capture{{"7.8", 0, 3}, {"7", 0, 1}, {".8", 1, 3}, {"8", 2, 3}}},
			{"optionalAbsent", `^(?<stem>[0-9]+)(\.([0-9]+))?$`, "7", []capture{{"7", 0, 1}, {"7", 0, 1}, {nil, -1, -1}, {nil, -1, -1}}},
			{"interleaved", `^(?<first>[0-9]+)(\.([0-9]+))?(\.(?<last>[0-9]+))?$`, "7.8.9", []capture{{"7.8.9", 0, 5}, {"7", 0, 1}, {".8", 1, 3}, {"8", 2, 3}, {".9", 3, 5}, {"9", 4, 5}}},
			{"plainControl", `^(a)(b)$`, "ab", []capture{{"ab", 0, 2}, {"a", 0, 1}, {"b", 1, 2}}},
			{"namedOnlyControl", `^(?<head>a)(?<tail>b)$`, "ab", []capture{{"ab", 0, 2}, {"a", 0, 1}, {"b", 1, 2}}},
			{"privateGraphemeCapture", `^(\X)(?<tail>b)(c)$`, "a\u0301bc", []capture{{"a\u0301bc", 0, 4}, {"a\u0301", 0, 2}, {"b", 2, 3}, {"c", 3, 4}}},
			{"scopedFlagsAndLookahead", `(?i:(?<word>a))(b)(?=c)c`, "Abc", []capture{{"Abc", 0, 3}, {"A", 0, 1}, {"b", 1, 2}}},
		} {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				var source strings.Builder
				fmt.Fprintf(&source, "Matcher m=Pattern.compile('%s').matcher('%s');\nSystem.assert(m.matches());\nSystem.assertEquals(%d,m.groupCount());\n", mixedCaptureQuote(tc.pattern), mixedCaptureQuote(tc.input), len(tc.groups)-1)
				for group, expected := range tc.groups {
					literal := "null"
					if expected.value != nil {
						literal = "'" + mixedCaptureQuote(expected.value.(string)) + "'"
					}
					fmt.Fprintf(&source, "System.assertEquals(%s,m.group(%d),'group %d');\nSystem.assertEquals(%d,m.start(%d),'start %d');\nSystem.assertEquals(%d,m.end(%d),'end %d');\n", literal, group, group, expected.start, group, group, expected.end, group, group)
				}
				program, err := CompileAnonymousWithOptions(source.String(), CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := Execute(program, nil); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestMixedCaptureReplacementUsesOpeningOrder(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, tc := range []struct {
			name, replacement, expected string
		}{
			{"numbered", "$2$1", "ba"},
			{"namedAndNumbered", "${head}-$2", "a-b"},
		} {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				source := fmt.Sprintf("System.assertEquals('%s',Pattern.compile('(?<head>a)(b)').matcher('ab').replaceAll('%s'));", mixedCaptureQuote(tc.expected), mixedCaptureQuote(tc.replacement))
				program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := Execute(program, nil); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestMixedCaptureOrderPreservesUnmixedBackreferences(t *testing.T) {
	program, err := CompileAnonymous(`
Matcher numbered=Pattern.compile('^(a)\\1$').matcher('aa');
System.assert(numbered.matches());
System.assertEquals(1,numbered.groupCount());
System.assertEquals('a',numbered.group(1));
Matcher named=Pattern.compile('^(?<head>a)\\k<head>$').matcher('aa');
System.assert(named.matches());
System.assertEquals(1,named.groupCount());
System.assertEquals('a',named.group(1));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func mixedCaptureQuote(value string) string {
	return strings.NewReplacer("\\", "\\\\", "'", "\\'", "\n", "\\n", "\r", "\\r", "\t", "\\t").Replace(value)
}
