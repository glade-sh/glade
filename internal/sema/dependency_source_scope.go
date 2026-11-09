package sema

import (
	"strings"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/typesys"
)

// sourceDependencyScope answers whether a diagnostic's file can depend on the
// loaded source-backed dependencies. A source dependency makes resolution
// uncertain only for code that reaches it: a spelling qualified by its
// namespace, one of its type names, or a supertype chain that passes through
// it. A class that names none of these fails to compile the same way with or
// without the dependency, so its errors stay errors.
type sourceDependencyScope struct {
	index     typesys.Index
	sources   *semaSources
	tokens    map[string]bool
	fileTypes map[string][]typesys.TypeSymbol
	byName    map[string]typesys.TypeSymbol
	verdicts  map[string]bool
}

func newSourceDependencyScope(index typesys.Index, a *Analyzer) *sourceDependencyScope {
	scope := &sourceDependencyScope{
		index:     index,
		tokens:    make(map[string]bool),
		fileTypes: make(map[string][]typesys.TypeSymbol),
		byName:    make(map[string]typesys.TypeSymbol),
		verdicts:  make(map[string]bool),
	}
	if a != nil {
		scope.sources = a.sources
	}
	for _, dep := range index.Dependencies {
		if dep.Status == "loaded" && dep.SourceRoot != "" && !semaDependencyBackedByArtifact(index, dep.Namespace) {
			scope.tokens[normalizeName(dep.Namespace)] = true
		}
	}
	for _, typ := range index.Types {
		if typ.Dependency && !typ.Artifact {
			scope.tokens[normalizeName(typ.Name)] = true
			if typ.Namespace != "" {
				scope.tokens[normalizeName(typ.Namespace)] = true
			}
			continue
		}
		if typ.Dependency {
			continue
		}
		scope.fileTypes[typ.File] = append(scope.fileTypes[typ.File], typ)
		scope.byName[normalizeName(typ.Name)] = typ
	}
	return scope
}

// localToFile reports that the diagnostic's source file never reaches a
// loaded source dependency. It answers false whenever the source cannot be
// read, so unattributable diagnostics keep the dependency downgrade.
func (s *sourceDependencyScope) localToFile(d diagnostic.Diagnostic) bool {
	// Only the plain local-resolution misses are attributable to a file's own
	// names. Other codes carry uncertainty about the platform, not the package.
	switch d.Code {
	case "GLADESEMA006", "GLADESEMA008", "GLADESEMA009":
	default:
		return false
	}
	if s == nil || s.sources == nil || d.File == "" || len(s.tokens) == 0 {
		return false
	}
	if verdict, ok := s.verdicts[d.File]; ok {
		return verdict
	}
	verdict := s.fileIsLocal(d.File)
	s.verdicts[d.File] = verdict
	return verdict
}

func (s *sourceDependencyScope) fileIsLocal(file string) bool {
	types := s.fileTypes[file]
	if len(types) == 0 {
		return false
	}
	for _, typ := range types {
		text, ok := s.sources.rawForType(typ)
		if !ok {
			return false
		}
		if s.textNamesDependency(text) || s.inheritsDependency(typ, map[string]bool{}) {
			return false
		}
	}
	return true
}

func (s *sourceDependencyScope) textNamesDependency(text string) bool {
	start := -1
	for i := 0; i <= len(text); i++ {
		word := i < len(text) && (text[i] == '_' || text[i] >= '0' && text[i] <= '9' || text[i] >= 'a' && text[i] <= 'z' || text[i] >= 'A' && text[i] <= 'Z')
		if word {
			if start < 0 {
				start = i
			}
			continue
		}
		if start >= 0 {
			if s.tokens[normalizeName(text[start:i])] {
				return true
			}
			start = -1
		}
	}
	return false
}

func (s *sourceDependencyScope) inheritsDependency(typ typesys.TypeSymbol, seen map[string]bool) bool {
	key := normalizeName(typ.Name)
	if seen[key] {
		return false
	}
	seen[key] = true
	supers := append([]string{typ.SuperClass}, typ.Interfaces...)
	for _, name := range supers {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if s.textNamesDependency(name) {
			return true
		}
		if parent, ok := s.byName[normalizeName(name)]; ok && s.inheritsDependency(parent, seen) {
			return true
		}
	}
	return false
}
