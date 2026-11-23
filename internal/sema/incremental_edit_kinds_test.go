package sema

import (
	"math/rand"
	"regexp"
	"strconv"
	"strings"
)

// Edit kinds beyond inserted statements. Each picks a site at random and
// reports false when the source has none. Some keep the index identical (a
// body-only edit, reused under SkipMentionRechecks); others change it and
// take the full analysis; the equality sweep and the mention evidence cover
// both.
func init() {
	incrementalEdits = append(incrementalEdits,
		incrementalEdit{"field-initializer", incrementalEditFieldInitializer},
		incrementalEdit{"constant-literal", incrementalEditConstantLiteral},
		incrementalEdit{"static-block-body", incrementalEditBlockBody(regexp.MustCompile(`\bstatic\s*\{`))},
		incrementalEdit{"accessor-body", incrementalEditBlockBody(regexp.MustCompile(`(?i)\b[gs]et\s*\{`))},
		incrementalEdit{"inner-type-body", incrementalEditInnerTypeBody},
		incrementalEdit{"delete-statement", incrementalEditDeleteStatement},
		incrementalEdit{"soql-field-list", incrementalEditSOQLFieldList},
	)
}

// incrementalBraceDepth is the brace depth at offset, ignoring braces in
// string literals and comments.
func incrementalBraceDepth(source string, offset int) int {
	depth := 0
	for i := 0; i < offset && i < len(source); i++ {
		switch c := source[i]; {
		case c == '\'':
			for i++; i < len(source) && source[i] != '\''; i++ {
				if source[i] == '\\' {
					i++
				}
			}
		case c == '/' && i+1 < len(source) && source[i+1] == '/':
			for i < len(source) && source[i] != '\n' {
				i++
			}
		case c == '/' && i+1 < len(source) && source[i+1] == '*':
			end := strings.Index(source[i+2:], "*/")
			if end < 0 {
				return depth
			}
			i += end + 3
		case c == '{':
			depth++
		case c == '}':
			depth--
		}
	}
	return depth
}

var incrementalFieldInitializer = regexp.MustCompile(`(?m)^[ \t]*(?:(?:public|private|protected|global|static|final|transient)\s+)+[\w.<>, ]+\s+\w+\s*=\s*([^;{}]+);`)

func incrementalClassLevelMatches(source string, re *regexp.Regexp) [][]int {
	var out [][]int
	for _, match := range re.FindAllStringSubmatchIndex(source, -1) {
		if incrementalBraceDepth(source, match[0]) == 1 {
			out = append(out, match)
		}
	}
	return out
}

// incrementalEditFieldInitializer rewrites one class-level initializer value.
func incrementalEditFieldInitializer(source string, rng *rand.Rand) (string, bool) {
	matches := incrementalClassLevelMatches(source, incrementalFieldInitializer)
	if len(matches) == 0 {
		return source, false
	}
	match := matches[rng.Intn(len(matches))]
	value := strings.TrimSpace(source[match[2]:match[3]])
	var next string
	switch {
	case strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'"):
		next = strings.TrimSuffix(value, "'") + "zz'"
	case value == "null":
		next = "(null)"
	default:
		next = "(" + value + ")"
	}
	return source[:match[2]] + next + source[match[3]:], true
}

var incrementalIntegerLiteral = regexp.MustCompile(`\b\d+\b`)

// incrementalEditConstantLiteral changes an integer literal in a class-level
// initializer, so the constant's value changes.
func incrementalEditConstantLiteral(source string, rng *rand.Rand) (string, bool) {
	matches := incrementalClassLevelMatches(source, incrementalFieldInitializer)
	var sites [][2]int
	for _, match := range matches {
		for _, literal := range incrementalIntegerLiteral.FindAllStringIndex(source[match[2]:match[3]], -1) {
			sites = append(sites, [2]int{match[2] + literal[0], match[2] + literal[1]})
		}
	}
	if len(sites) == 0 {
		return source, false
	}
	site := sites[rng.Intn(len(sites))]
	n, err := strconv.Atoi(source[site[0]:site[1]])
	if err != nil {
		return source, false
	}
	return source[:site[0]] + strconv.Itoa(n+1) + source[site[1]:], true
}

// incrementalEditBlockBody inserts a statement at the start of a block that
// opener matches.
func incrementalEditBlockBody(opener *regexp.Regexp) func(string, *rand.Rand) (string, bool) {
	return func(source string, rng *rand.Rand) (string, bool) {
		matches := opener.FindAllStringIndex(source, -1)
		if len(matches) == 0 {
			return source, false
		}
		at := matches[rng.Intn(len(matches))][1]
		return source[:at] + "\n        Integer zzBlock = 1; zzBlock++;\n" + source[at:], true
	}
}

var incrementalInnerType = regexp.MustCompile(`(?i)\b(?:class|enum|interface)\s+\w+[^;{]*\{`)

// incrementalEditInnerTypeBody inserts a statement into a method body of an
// inner type.
func incrementalEditInnerTypeBody(source string, rng *rand.Rand) (string, bool) {
	var sites []int
	for _, inner := range incrementalInnerType.FindAllStringIndex(source, -1) {
		if incrementalBraceDepth(source, inner[0]) != 1 {
			continue
		}
		depth := 2
		for i := inner[1]; i < len(source) && depth >= 2; i++ {
			switch source[i] {
			case '{':
				j := i - 1
				for j >= 0 && strings.ContainsRune(" \t\r\n", rune(source[j])) {
					j--
				}
				if depth == 2 && j >= 0 && source[j] == ')' {
					sites = append(sites, i+1)
				}
				depth++
			case '}':
				depth--
			}
		}
	}
	if len(sites) == 0 {
		return source, false
	}
	at := sites[rng.Intn(len(sites))]
	return source[:at] + "\n            Integer zzInner = 1; zzInner++;\n" + source[at:], true
}

// incrementalEditDeleteStatement removes the first statement of a method
// body. A deleted assignment or query can remove a hydrated SObject field, or
// a later use can become an error.
func incrementalEditDeleteStatement(source string, rng *rand.Rand) (string, bool) {
	at := incrementalBodyOpen(source, rng)
	if at < 0 {
		return source, false
	}
	depth := 0
	for i := at; i < len(source); i++ {
		switch source[i] {
		case '{':
			depth++
		case '}':
			if depth == 0 {
				return source, false
			}
			depth--
		case '\'':
			for i++; i < len(source) && source[i] != '\''; i++ {
				if source[i] == '\\' {
					i++
				}
			}
		case ';':
			if depth == 0 {
				if strings.TrimSpace(source[at:i]) == "" {
					return source, false
				}
				return source[:at] + "\n" + source[i+1:], true
			}
		}
	}
	return source, false
}

var incrementalSOQLSelect = regexp.MustCompile(`(?i)\[\s*SELECT\s+`)

// incrementalEditSOQLFieldList changes the field list of an inline query:
// it drops the first field when there are several, or adds Id.
func incrementalEditSOQLFieldList(source string, rng *rand.Rand) (string, bool) {
	matches := incrementalSOQLSelect.FindAllStringIndex(source, -1)
	if len(matches) == 0 {
		return source, false
	}
	at := matches[rng.Intn(len(matches))][1]
	rest := source[at:]
	from := strings.Index(strings.ToUpper(rest), " FROM ")
	if from < 0 {
		return source, false
	}
	fields := rest[:from]
	if comma := strings.Index(fields, ","); comma >= 0 && !strings.Contains(fields[:comma], "(") {
		return source[:at] + strings.TrimLeft(fields[comma+1:], " ") + rest[from:], true
	}
	return source[:at] + "Id, " + rest, true
}
