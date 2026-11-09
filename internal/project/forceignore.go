package project

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ForceIgnoreFileName is the Salesforce CLI ignore file. A source deploy skips
// every path it matches.
const ForceIgnoreFileName = ".forceignore"

// forceIgnoreFile is one parsed .forceignore. Patterns use gitignore syntax and
// are relative to the directory that holds the file.
type forceIgnoreFile struct {
	dir string
	// prefix is dir plus a trailing separator, for the per-file containment check.
	prefix string
	rules  []forceIgnoreRule
	// dirs memoizes whether a directory under dir is ignored, keyed by its
	// slash-separated path relative to dir.
	dirs map[string]bool
}

type forceIgnoreRule struct {
	negate  bool
	dirOnly bool
	// basename rules have no slash and match the last path element at any
	// depth, as gitignore does.
	basename bool
	// literal is a lowercase ASCII run every match must contain; it lets most
	// paths skip the regexp.
	literal string
	// prefix and suffix are the lowercase ASCII runs before the first and
	// after the last glob character; a match must start and end with them.
	prefix string
	suffix string
	re     *regexp.Regexp
}

// forceIgnoreSet answers deploy-equivalent ignore questions for package
// source. A file is governed by the nearest .forceignore in its own directory
// or an ancestor; that file alone decides, with paths relative to its
// directory. This matches a default `sf project deploy start` against a
// source-tracked org: a nearer .forceignore replaces the outer one for the
// files below it.
type forceIgnoreSet struct {
	nearest map[string]*forceIgnoreFile
	loaded  map[string]*forceIgnoreFile
}

func newForceIgnoreSet() *forceIgnoreSet {
	return &forceIgnoreSet{
		nearest: make(map[string]*forceIgnoreFile),
		loaded:  make(map[string]*forceIgnoreFile),
	}
}

// governing returns the nearest .forceignore for dir, or nil when none exists.
// dir must be clean; filepath.Dir results are.
func (s *forceIgnoreSet) governing(dir string) *forceIgnoreFile {
	if fi, ok := s.nearest[dir]; ok {
		return fi
	}
	var fi *forceIgnoreFile
	if parsed, ok := s.load(dir); ok {
		fi = parsed
	} else if parent := filepath.Dir(dir); parent != dir {
		fi = s.governing(parent)
	}
	s.nearest[dir] = fi
	return fi
}

func (s *forceIgnoreSet) load(dir string) (*forceIgnoreFile, bool) {
	if fi, ok := s.loaded[dir]; ok {
		return fi, fi != nil
	}
	path := filepath.Join(dir, ForceIgnoreFileName)
	data, err := os.ReadFile(path) // #nosec G304 -- path is a fixed filename under a project source directory.
	if err != nil {
		s.loaded[dir] = nil
		return nil, false
	}
	fi := newForceIgnoreFile(dir, parseForceIgnore(string(data)))
	s.loaded[dir] = fi
	return fi, true
}

func newForceIgnoreFile(dir string, rules []forceIgnoreRule) *forceIgnoreFile {
	return &forceIgnoreFile{dir: dir, prefix: dir + string(filepath.Separator), rules: rules, dirs: make(map[string]bool)}
}

// Files lists the .forceignore files that governed at least one lookup.
func (s *forceIgnoreSet) Files() []string {
	if s == nil {
		return nil
	}
	var files []string
	for dir, fi := range s.loaded {
		if fi != nil {
			files = append(files, filepath.Join(dir, ForceIgnoreFileName))
		}
	}
	return files
}

// Ignored reports whether a source file is excluded from a deploy. A file is
// also excluded when its paired -meta.xml is ignored, because the deploy then
// drops the whole component.
func (s *forceIgnoreSet) Ignored(path string) bool {
	if s == nil {
		return false
	}
	fi := s.governing(filepath.Dir(path))
	if fi == nil || len(fi.rules) == 0 {
		return false
	}
	if !strings.HasPrefix(path, fi.prefix) {
		return false
	}
	rel := filepath.ToSlash(path[len(fi.prefix):])
	if dir, _, ok := reverseCutSlash(rel); ok && fi.dirIgnored(dir) {
		return true
	}
	lower := strings.ToLower(rel)
	if fi.match(rel, lower, false) {
		return true
	}
	if !strings.HasSuffix(lower, "-meta.xml") {
		return fi.match(rel+"-meta.xml", lower+"-meta.xml", false)
	}
	return false
}

func reverseCutSlash(rel string) (string, string, bool) {
	i := strings.LastIndexByte(rel, '/')
	if i < 0 {
		return "", rel, false
	}
	return rel[:i], rel[i+1:], true
}

// dirIgnored applies gitignore precedence for directories: once a parent is
// excluded nothing below it can be re-included.
func (fi *forceIgnoreFile) dirIgnored(dir string) bool {
	if ignored, ok := fi.dirs[dir]; ok {
		return ignored
	}
	ignored := false
	if parent, _, ok := reverseCutSlash(dir); ok && fi.dirIgnored(parent) {
		ignored = true
	} else {
		ignored = fi.match(dir, strings.ToLower(dir), true)
	}
	fi.dirs[dir] = ignored
	return ignored
}

// match returns the outcome of the last rule that matches rel. lower is rel
// lowercased, for the literal prefilter.
func (fi *forceIgnoreFile) match(rel, lower string, isDir bool) bool {
	ignored := false
	base, lowerBase := rel, lower
	if i := strings.LastIndexByte(rel, '/'); i >= 0 {
		base, lowerBase = rel[i+1:], lower[i+1:]
	}
	for _, rule := range fi.rules {
		if rule.dirOnly && !isDir {
			continue
		}
		if ignored == !rule.negate {
			continue
		}
		subject, lowerSubject := rel, lower
		if rule.basename {
			subject, lowerSubject = base, lowerBase
		}
		if !strings.HasPrefix(lowerSubject, rule.prefix) || !strings.HasSuffix(lowerSubject, rule.suffix) {
			continue
		}
		if rule.literal != "" && !strings.Contains(lowerSubject, rule.literal) {
			continue
		}
		if rule.re.MatchString(subject) {
			ignored = !rule.negate
		}
	}
	return ignored
}

// parseForceIgnore compiles gitignore-style lines. Matching is
// case-insensitive, as the Salesforce CLI matches.
func parseForceIgnore(content string) []forceIgnoreRule {
	var rules []forceIgnoreRule
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSuffix(line, "\r")
		line = trimUnescapedTrailingSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		rule := forceIgnoreRule{}
		if strings.HasPrefix(line, "!") {
			rule.negate = true
			line = line[1:]
		} else if strings.HasPrefix(line, `\!`) || strings.HasPrefix(line, `\#`) {
			line = line[1:]
		}
		if strings.HasSuffix(line, "/") {
			rule.dirOnly = true
			line = strings.TrimRight(line, "/")
		}
		if line == "" {
			continue
		}
		rule.basename = !strings.Contains(line, "/")
		line = strings.TrimPrefix(line, "/")
		rule.literal = globLiteral(line)
		rule.prefix, rule.suffix = globAffixes(line)
		re, err := regexp.Compile("(?i)^" + globToRegexp(line) + "$")
		if err != nil {
			continue
		}
		rule.re = re
		rules = append(rules, rule)
	}
	return rules
}

// globLiteral returns the longest run of plain ASCII characters in pattern
// that no slash or glob interrupts,
// lowercased, or "" when the pattern has none worth checking.
func globLiteral(pattern string) string {
	best, run := "", strings.Builder{}
	flush := func() {
		if run.Len() > len(best) {
			best = run.String()
		}
		run.Reset()
	}
	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		switch {
		case c == '*' || c == '?' || c == '[' || c == '\\' || c == '/' || c >= 0x80:
			flush()
			if c == '[' {
				if end := strings.IndexByte(pattern[i+1:], ']'); end >= 0 {
					i += end + 1
				}
			} else if c == '\\' {
				i++
			}
		default:
			run.WriteByte(c)
		}
	}
	flush()
	if len(best) < 2 {
		return ""
	}
	return strings.ToLower(best)
}

// globAffixes returns the lowercase plain ASCII text before the first and
// after the last glob, bracket, escape or non-ASCII byte. A pattern with no
// glob is all prefix.
func globAffixes(pattern string) (string, string) {
	special := func(c byte) bool {
		return c == '*' || c == '?' || c == '[' || c == ']' || c == '\\' || c >= 0x80
	}
	first := 0
	for first < len(pattern) && !special(pattern[first]) {
		first++
	}
	if first == len(pattern) {
		return strings.ToLower(pattern), ""
	}
	last := len(pattern)
	for last > first && !special(pattern[last-1]) {
		last--
	}
	// A `**/` before the suffix may match zero directories, so the slash
	// that follows it is optional.
	return strings.ToLower(pattern[:first]), strings.ToLower(strings.TrimPrefix(pattern[last:], "/"))
}

func trimUnescapedTrailingSpace(line string) string {
	for strings.HasSuffix(line, " ") || strings.HasSuffix(line, "\t") {
		if strings.HasSuffix(line, `\ `) {
			return line[:len(line)-2] + " "
		}
		line = line[:len(line)-1]
	}
	return line
}

func globToRegexp(pattern string) string {
	segments := strings.Split(pattern, "/")
	var b strings.Builder
	for i, segment := range segments {
		last := i == len(segments)-1
		if segment == "**" {
			if last {
				if i == 0 {
					b.WriteString(".*")
				} else {
					b.WriteString(".+")
				}
			} else {
				// Zero or more directories.
				b.WriteString("(?:.*/)?")
			}
			continue
		}
		b.WriteString(globSegmentToRegexp(segment))
		if !last {
			b.WriteByte('/')
		}
	}
	return b.String()
}

func globSegmentToRegexp(segment string) string {
	var b strings.Builder
	for i := 0; i < len(segment); i++ {
		c := segment[i]
		switch c {
		case '*':
			for i+1 < len(segment) && segment[i+1] == '*' {
				i++
			}
			b.WriteString("[^/]*")
		case '?':
			b.WriteString("[^/]")
		case '\\':
			if i+1 < len(segment) {
				i++
				b.WriteString(regexp.QuoteMeta(string(segment[i])))
			}
		case '[':
			end := strings.IndexByte(segment[i+1:], ']')
			if end < 0 {
				b.WriteString(`\[`)
				continue
			}
			class := segment[i+1 : i+1+end]
			i += end + 1
			b.WriteByte('[')
			if strings.HasPrefix(class, "!") || strings.HasPrefix(class, "^") {
				b.WriteByte('^')
				class = class[1:]
			}
			for j := 0; j < len(class); j++ {
				switch class[j] {
				case '\\', '[', ']', '^':
					b.WriteByte('\\')
				}
				b.WriteByte(class[j])
			}
			b.WriteByte(']')
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	return b.String()
}
