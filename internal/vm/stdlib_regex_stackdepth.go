package vm

import (
	"fmt"
	"strings"

	"github.com/dlclark/regexp2"
)

// Java matches a quantified group with a variable-length body through a
// recursive Loop node. Salesforce runs the match on a bounded stack, so a long
// run of iterations ends in StringException "Stack Overflow" (catchable). The
// regex_java_divergence rows record the boundary: (a|b)* succeeds at 1,200
// characters and overflows at 1,300. Groups with a fixed-length body, possessive
// groups and plain character repeats never recurse and never overflow.
const (
	javaRegexLoopIterationLimit = 1250
	// regexLoopMinInput is the shortest subject text that can reach the limit:
	// each iteration consumes at least one character.
	regexLoopMinInput = javaRegexLoopIterationLimit
)

// instrumentJavaLoopGroups wraps each recursive-loop group in a private
// capture. The capture records one entry per iteration, which the plan reads
// after a successful match. The pattern must already be a valid Java pattern.
func instrumentJavaLoopGroups(normalized string) (string, []string) {
	loops := javaRegexLoopGroups(normalized)
	if len(loops) == 0 {
		return normalized, nil
	}
	type insertion struct {
		at   int
		text string
		open bool
	}
	var inserts []insertion
	var names []string
	prefix := "__gladeLoop"
	for strings.Contains(normalized, prefix) {
		prefix += "X"
	}
	for i, loop := range loops {
		name := fmt.Sprintf("%s%d", prefix, i+1)
		names = append(names, name)
		inserts = append(inserts, insertion{at: loop.start, text: "(?<" + name + ">", open: true}, insertion{at: loop.end, text: ")"})
	}
	runes := []rune(normalized)
	var out strings.Builder
	for pos := 0; pos <= len(runes); pos++ {
		// Closing parentheses first, then openings, so adjacent groups nest correctly.
		for _, in := range inserts {
			if in.at == pos && !in.open {
				out.WriteString(in.text)
			}
		}
		for _, in := range inserts {
			if in.at == pos && in.open {
				out.WriteString(in.text)
			}
		}
		if pos < len(runes) {
			out.WriteRune(runes[pos])
		}
	}
	return out.String(), names
}

// checkStackDepth reports Java's stack overflow for a match that ran a
// recursive loop for too many iterations.
func (p *regexp2Plan) checkStackDepth(match *regexp2.Match) error {
	for _, name := range p.loopGroups {
		if group := match.GroupByName(name); group != nil && len(group.Captures) > javaRegexLoopIterationLimit {
			return newExceptionError("System.StringException", "Stack Overflow")
		}
	}
	return nil
}
