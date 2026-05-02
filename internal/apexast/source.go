package apexast

import (
	"net/url"
	"path/filepath"
	"strings"

	"github.com/open-aer/oaer/internal/diagnostic"
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

func (m LineMap) Position(offset int) diagnostic.Position {
	if offset < 0 {
		offset = 0
	}
	line := 0
	for line+1 < len(m.starts) && m.starts[line+1] <= offset {
		line++
	}
	return diagnostic.Position{
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
