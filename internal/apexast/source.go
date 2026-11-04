package apexast

import (
	"net/url"
	"path/filepath"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/diagnostic"
)

type LineMap struct {
	starts []int
	length int
}

func NewLineMap(source string) LineMap {
	starts := []int{0}
	for i, r := range source {
		if r == '\n' {
			starts = append(starts, i+1)
		}
	}
	return LineMap{starts: starts, length: len(source)}
}

func (m LineMap) Position(offset int) diagnostic.Position {
	if offset < 0 {
		offset = 0
	}
	if offset > m.length {
		offset = m.length
	}
	line := sort.Search(len(m.starts), func(i int) bool { return m.starts[i] > offset }) - 1
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
