package apexast

import (
	"net/url"
	"path/filepath"
	"strings"
)

type LineMap struct {
	starts []int
}

func NewLineMap(source string) LineMap {
	starts := []int{0}
	for i, r := range source {
		if r == '\n' {
			starts = append(starts, i+1)
		}
	}
	return LineMap{starts: starts}
}

func (m LineMap) Position(offset int) Position {
	if offset < 0 {
		offset = 0
	}
	line := 0
	for line+1 < len(m.starts) && m.starts[line+1] <= offset {
		line++
	}
	return Position{
		Line:   line + 1,
		Column: offset - m.starts[line] + 1,
		Offset: offset,
	}
}

func FileURI(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	u := url.URL{Scheme: "file", Path: filepath.ToSlash(abs)}
	if !strings.HasPrefix(u.Path, "/") {
		u.Path = "/" + u.Path
	}
	return u.String()
}

func excerpt(source string, line int) string {
	if line <= 0 {
		return ""
	}
	start := 0
	current := 1
	for i, r := range source {
		if current == line && r == '\n' {
			return source[start:i]
		}
		if r == '\n' {
			current++
			start = i + 1
		}
	}
	if current == line {
		return source[start:]
	}
	return ""
}
