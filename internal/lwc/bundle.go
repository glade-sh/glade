package lwc

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/project"
)

type Bundle struct {
	Name     string
	Dir      string
	JSFile   string
	HTMLFile string
	CSSFile  string
	MetaFile string
}

type Index struct {
	byName map[string]Bundle
}

func BuildIndex(p project.Project) (Index, error) {
	groups := map[string]*Bundle{}
	add := func(path string, assign func(b *Bundle, path string)) {
		dir := filepath.Dir(path)
		name := filepath.Base(dir)
		b, ok := groups[name]
		if !ok {
			b = &Bundle{Name: name, Dir: dir}
			groups[name] = b
		}
		assign(b, path)
	}
	for _, path := range p.LWCFiles {
		add(path, func(b *Bundle, path string) { b.JSFile = path })
	}
	for _, path := range p.LWCHTMLFiles {
		add(path, func(b *Bundle, path string) { b.HTMLFile = path })
	}
	for _, path := range p.LWCCSSFiles {
		add(path, func(b *Bundle, path string) { b.CSSFile = path })
	}
	for _, path := range p.LWCMetaFiles {
		add(path, func(b *Bundle, path string) { b.MetaFile = path })
	}
	idx := Index{byName: make(map[string]Bundle, len(groups))}
	for name, b := range groups {
		idx.byName[strings.ToLower(name)] = *b
	}
	return idx, nil
}

func (idx Index) Bundle(name string) (Bundle, bool) {
	b, ok := idx.byName[strings.ToLower(strings.TrimSpace(name))]
	return b, ok
}

func (idx Index) Names() []string {
	names := make([]string, 0, len(idx.byName))
	for name := range idx.byName {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (b Bundle) ReadHTML() (string, error) {
	if b.HTMLFile == "" {
		return "", os.ErrNotExist
	}
	data, err := os.ReadFile(b.HTMLFile)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
