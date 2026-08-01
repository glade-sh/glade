package vm

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/dlclark/regexp2"
	"github.com/rivo/uniseg"
)

type graphemeCluster struct {
	text      string
	startByte int
	endByte   int
}

type graphemeBoundaryTable struct {
	clusters      []graphemeCluster
	boundaryBytes map[int]bool
}

type regexp2Plan struct {
	source             string
	re                 *regexp2.Regexp
	grapheme           *graphemeBoundaryTable
	internalGroupNames map[string]bool
	publicGroupNumbers []int
}

func buildGraphemeBoundaryTable(input string) *graphemeBoundaryTable {
	table := &graphemeBoundaryTable{boundaryBytes: map[int]bool{0: true}}
	pos := 0
	g := uniseg.NewGraphemes(input)
	for g.Next() {
		cluster := g.Str()
		next := pos + len(cluster)
		table.clusters = append(table.clusters, graphemeCluster{text: cluster, startByte: pos, endByte: next})
		table.boundaryBytes[next] = true
		pos = next
	}
	table.boundaryBytes[len(input)] = true
	return table
}

func (g *graphemeBoundaryTable) isBoundaryByte(pos int) bool {
	if g == nil {
		return true
	}
	return g.boundaryBytes[pos]
}

func compileRegexp2PlanForInput(callee, source, input string) (*regexp2Plan, error) {
	regexp2Source, err := compileRegexp2Source(callee, source)
	if err != nil {
		return nil, err
	}
	table := buildGraphemeBoundaryTable(input)
	internal := map[string]bool{}
	regexp2Source = rewriteGraphemeTokensForRegexp2(regexp2Source, table, internal)
	re, err := regexp2.Compile(regexp2Source, regexp2.None)
	if err != nil {
		return nil, newPatternSyntaxExceptionError(source, err)
	}
	re.MatchTimeout = regexp2MatchTimeout
	return &regexp2Plan{
		source:             regexp2Source,
		re:                 re,
		grapheme:           table,
		internalGroupNames: internal,
		publicGroupNumbers: regexp2PublicGroupNumbers(re, internal),
	}, nil
}

func regexp2CompileSourceForSyntax(source string) string {
	table := buildGraphemeBoundaryTable("a")
	internal := map[string]bool{}
	return rewriteGraphemeTokensForRegexp2(source, table, internal)
}

func rewriteGraphemeTokensForRegexp2(source string, table *graphemeBoundaryTable, internal map[string]bool) string {
	if !strings.Contains(source, `\X`) && !strings.Contains(source, `\b{g}`) {
		return source
	}
	var out strings.Builder
	inClass := false
	nextName := 0
	clusterPattern := graphemeClusterAlternation(table)
	for i := 0; i < len(source); i++ {
		ch := source[i]
		switch ch {
		case '\\':
			if i+1 >= len(source) {
				out.WriteByte(ch)
				continue
			}
			next := source[i+1]
			if !inClass && next == 'X' {
				name := nextInternalGraphemeGroupName(source, internal, "__gladeGX", &nextName)
				internal[name] = true
				out.WriteString("(?<")
				out.WriteString(name)
				out.WriteString(">")
				out.WriteString(clusterPattern)
				out.WriteByte(')')
				i++
				continue
			}
			if !inClass && next == 'b' && strings.HasPrefix(source[i:], `\b{g}`) {
				name := nextInternalGraphemeGroupName(source, internal, "__gladeGB", &nextName)
				internal[name] = true
				out.WriteString("(?<")
				out.WriteString(name)
				out.WriteString(">)")
				i += len(`\b{g}`) - 1
				continue
			}
			out.WriteByte(ch)
			i++
			out.WriteByte(source[i])
		case '[':
			inClass = true
			out.WriteByte(ch)
		case ']':
			inClass = false
			out.WriteByte(ch)
		default:
			out.WriteByte(ch)
		}
	}
	return out.String()
}

func nextInternalGraphemeGroupName(source string, internal map[string]bool, prefix string, next *int) string {
	for {
		name := fmt.Sprintf("%s%d", prefix, *next)
		*next = *next + 1
		if internal[name] || strings.Contains(source, "?<"+name+">") || strings.Contains(source, "?P<"+name+">") {
			continue
		}
		return name
	}
}

func graphemeClusterAlternation(table *graphemeBoundaryTable) string {
	if table == nil || len(table.clusters) == 0 {
		return `(?!)`
	}
	seen := map[string]bool{}
	clusters := make([]string, 0, len(table.clusters))
	for _, cluster := range table.clusters {
		if seen[cluster.text] {
			continue
		}
		seen[cluster.text] = true
		clusters = append(clusters, cluster.text)
	}
	sort.SliceStable(clusters, func(i, j int) bool {
		return len(clusters[i]) > len(clusters[j])
	})
	quoted := make([]string, 0, len(clusters))
	for _, cluster := range clusters {
		quoted = append(quoted, regexp.QuoteMeta(cluster))
	}
	return `(?:` + strings.Join(quoted, "|") + `)`
}

func regexp2PublicGroupNumbers(re *regexp2.Regexp, internal map[string]bool) []int {
	numbers := re.GetGroupNumbers()
	out := make([]int, 0, len(numbers))
	for _, number := range numbers {
		name := re.GroupNameFromNumber(number)
		if internal[name] {
			continue
		}
		out = append(out, number)
	}
	return out
}

func (p *regexp2Plan) findValidStartingAt(input string, startAt int) (*regexp2.Match, error) {
	match, err := p.re.FindStringMatchStartingAt(input, startAt)
	for match != nil && err == nil {
		if p.validGraphemeMatch(input, match) {
			return match, nil
		}
		match, err = p.re.FindNextMatch(match)
	}
	return nil, err
}

func (p *regexp2Plan) findNextValid(input string, prev *regexp2.Match) (*regexp2.Match, error) {
	match, err := p.re.FindNextMatch(prev)
	for match != nil && err == nil {
		if p.validGraphemeMatch(input, match) {
			return match, nil
		}
		match, err = p.re.FindNextMatch(match)
	}
	return nil, err
}

func (p *regexp2Plan) validGraphemeMatch(input string, match *regexp2.Match) bool {
	if p.grapheme == nil || len(p.internalGroupNames) == 0 {
		return true
	}
	for name := range p.internalGroupNames {
		group := match.GroupByName(name)
		if group == nil {
			continue
		}
		for _, capture := range group.Captures {
			startByte, err := byteIndexForRuneIndex(input, capture.Index)
			if err != nil {
				return false
			}
			endByte, err := byteIndexForRuneIndex(input, capture.Index+capture.Length)
			if err != nil {
				return false
			}
			if !p.grapheme.isBoundaryByte(startByte) || !p.grapheme.isBoundaryByte(endByte) {
				return false
			}
			if strings.HasPrefix(name, "__gladeGX") && startByte == endByte {
				return false
			}
		}
	}
	return true
}

func (p *regexp2Plan) matchByteIndices(input string, match *regexp2.Match, runeOffset int) ([]int, error) {
	indices := make([]int, 0, len(p.publicGroupNumbers)*2)
	for _, groupNumber := range p.publicGroupNumbers {
		group := match.GroupByNumber(groupNumber)
		if group == nil || len(group.Captures) == 0 {
			indices = append(indices, -1, -1)
			continue
		}
		startRune := runeOffset + group.Index
		endRune := startRune + group.Length
		startByte, err := byteIndexForRuneIndex(input, startRune)
		if err != nil {
			return nil, err
		}
		endByte, err := byteIndexForRuneIndex(input, endRune)
		if err != nil {
			return nil, err
		}
		indices = append(indices, startByte, endByte)
	}
	return indices, nil
}

func (p *regexp2Plan) publicGroupCount() int {
	count := len(p.publicGroupNumbers)
	if count == 0 {
		return 0
	}
	return count - 1
}

func (p *regexp2Plan) publicGroupNumber(group int) (int, bool) {
	if group < 0 || group >= len(p.publicGroupNumbers) {
		return 0, false
	}
	return p.publicGroupNumbers[group], true
}
