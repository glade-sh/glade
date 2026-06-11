package flagparse

import (
	"fmt"
	"strings"
)

type Flag struct {
	Name     string
	Short    string
	HasValue bool
}

type Parser struct {
	command          string
	flags            map[string]Flag
	aliases          map[string]string
	allowPositionals bool
}

type Result struct {
	values       map[string]string
	valueHistory map[string][]string
	bools        map[string]bool
	Positionals  []string
}

func New(command string) *Parser {
	return &Parser{
		command: command,
		flags:   make(map[string]Flag),
		aliases: make(map[string]string),
	}
}

func (p *Parser) Bool(name, short string) *Parser {
	return p.add(Flag{Name: normalizeLong(name), Short: normalizeShort(short)})
}

func (p *Parser) String(name, short string) *Parser {
	return p.add(Flag{Name: normalizeLong(name), Short: normalizeShort(short), HasValue: true})
}

func (p *Parser) AllowPositionals(allow bool) *Parser {
	p.allowPositionals = allow
	return p
}

func (p *Parser) add(flag Flag) *Parser {
	if flag.Name == "" {
		return p
	}
	p.flags[flag.Name] = flag
	if flag.Short != "" {
		p.aliases[flag.Short] = flag.Name
	}
	return p
}

func (p *Parser) Parse(args []string) (Result, error) {
	result := Result{
		values:       make(map[string]string),
		valueHistory: make(map[string][]string),
		bools:        make(map[string]bool),
	}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			result.Positionals = append(result.Positionals, args[i+1:]...)
			return result, nil
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			if !p.allowPositionals {
				return result, fmt.Errorf("unexpected argument %q", arg)
			}
			result.Positionals = append(result.Positionals, arg)
			continue
		}

		name, inlineValue, hasInlineValue := splitFlag(arg)
		canonical, ok := p.canonicalName(name)
		if !ok {
			return result, p.unknownFlag(name)
		}
		flag := p.flags[canonical]
		if flag.HasValue {
			value := inlineValue
			if !hasInlineValue {
				if i+1 >= len(args) {
					return result, fmt.Errorf("%s requires a value", canonical)
				}
				if strings.HasPrefix(args[i+1], "-") && args[i+1] != "-" {
					return result, fmt.Errorf("%s requires a value", canonical)
				}
				value = args[i+1]
				i++
			}
			result.values[canonical] = value
			result.valueHistory[canonical] = append(result.valueHistory[canonical], value)
			continue
		}
		if hasInlineValue {
			return result, fmt.Errorf("%s does not take a value", canonical)
		}
		result.bools[canonical] = true
	}
	return result, nil
}

func (r Result) String(name string) string {
	return r.values[normalizeLong(name)]
}

func (r Result) Strings(name string) []string {
	values := r.valueHistory[normalizeLong(name)]
	out := make([]string, len(values))
	copy(out, values)
	return out
}

func (r Result) Bool(name string) bool {
	return r.bools[normalizeLong(name)]
}

func (p *Parser) canonicalName(name string) (string, bool) {
	name = normalizeInputFlag(name)
	if _, ok := p.flags[name]; ok {
		return name, true
	}
	if canonical, ok := p.aliases[name]; ok {
		return canonical, true
	}
	return "", false
}

func (p *Parser) unknownFlag(name string) error {
	normalized := normalizeInputFlag(name)
	if suggestion := closestFlag(normalized, p.knownFlags()); suggestion != "" {
		return fmt.Errorf("unknown flag %q; did you mean %q?", name, suggestion)
	}
	return fmt.Errorf("unknown flag %q", name)
}

func (p *Parser) knownFlags() []string {
	out := make([]string, 0, len(p.flags)+len(p.aliases))
	for name, flag := range p.flags {
		out = append(out, name)
		if flag.Short != "" {
			out = append(out, flag.Short)
		}
	}
	return out
}

func splitFlag(arg string) (name, value string, hasValue bool) {
	if strings.HasPrefix(arg, "--") {
		if before, after, ok := strings.Cut(arg, "="); ok {
			return before, after, true
		}
		return arg, "", false
	}
	if strings.HasPrefix(arg, "-") && len(arg) > 2 {
		short := arg[:2]
		rest := arg[2:]
		if strings.HasPrefix(rest, "=") {
			return short, strings.TrimPrefix(rest, "="), true
		}
		return short, rest, true
	}
	return arg, "", false
}

func normalizeInputFlag(name string) string {
	if strings.HasPrefix(name, "--") {
		return name
	}
	if strings.HasPrefix(name, "-") {
		return name
	}
	if len(name) == 1 {
		return "-" + name
	}
	return "--" + name
}

func normalizeLong(name string) string {
	if name == "" {
		return ""
	}
	name = strings.TrimPrefix(name, "--")
	return "--" + name
}

func normalizeShort(name string) string {
	if name == "" {
		return ""
	}
	name = strings.TrimPrefix(name, "-")
	if len(name) != 1 {
		return ""
	}
	return "-" + name
}

func closestFlag(input string, candidates []string) string {
	best := ""
	bestDistance := 4
	for _, candidate := range candidates {
		distance := levenshtein(strings.TrimLeft(input, "-"), strings.TrimLeft(candidate, "-"))
		if distance < bestDistance {
			bestDistance = distance
			best = candidate
		}
	}
	return best
}

func Suggest(input string, candidates []string) string {
	best := ""
	bestDistance := 4
	for _, candidate := range candidates {
		distance := levenshtein(strings.ToLower(input), strings.ToLower(candidate))
		if distance < bestDistance {
			bestDistance = distance
			best = candidate
		}
	}
	return best
}

func levenshtein(a, b string) int {
	if a == b {
		return 0
	}
	if a == "" {
		return len(b)
	}
	if b == "" {
		return len(a)
	}
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := 0; j <= len(b); j++ {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 0
			if a[i-1] != b[j-1] {
				cost = 1
			}
			cur[j] = minInt(cur[j-1]+1, prev[j]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}

func minInt(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}
