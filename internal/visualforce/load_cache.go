package visualforce

import (
	"strings"
	"sync/atomic"

	"github.com/glade-sh/glade/internal/project"
)

// loadCache memoizes the project-wide work of one strict load sequence: the
// Apex declaration scan behind expression validation, the controller compile
// gate, the component and alternative-rendering project checks, and the parsed
// page and component index. The per-page fallback in pageNames loads the same
// project once per page; without the cache every load parsed every Apex file
// again (and compiled the project when no ControllerCompilation matched), so a
// project with one bad page paid pages x classes. Entries are keyed by the
// inputs they depend on, so a load with a different file set recomputes.
//
// A cache belongs to one call chain and is never shared across goroutines.
type loadCache struct {
	sources    memo[*expressionValidationSources]
	components memo[struct{}]
	alternates memo[struct{}]
	parsed     memo[parsedIndex]
	rejected   memo[map[string]bool]
}

type memo[T any] struct {
	done  bool
	key   string
	value T
	err   error
}

func (m *memo[T]) get(key string, compute func() (T, error)) (T, error) {
	if !m.done || m.key != key {
		m.value, m.err = compute()
		m.key = key
		m.done = true
	}
	return m.value, m.err
}

type parsedIndex struct {
	pages      []Page
	components []Component
}

// apexSourceParses counts Apex files parsed for expression validation and
// controller compiles run by the controller gate. Tests use them to prove that
// registering N pages over M classes parses each class once.
var (
	apexSourceParses   atomic.Uint64
	controllerCompiles atomic.Uint64
)

func apexFilesKey(p project.Project) string {
	return strings.Join(p.ApexFiles, "\x00")
}

func pageFilesKey(p project.Project) string {
	return strings.Join(p.VisualforcePageFiles, "\x00") + "\x01" + strings.Join(p.VisualforceComponentFiles, "\x00")
}

func pageAndEmailFilesKey(p project.Project) string {
	return pageFilesKey(p) + "\x01" + strings.Join(p.EmailTemplateFiles, "\x00")
}

// expressionSources returns the Apex declarations, resource registry and
// schema that expression validation reads, computed once per project source set.
func (c *loadCache) expressionSources(p project.Project) (*expressionValidationSources, error) {
	if c == nil {
		return loadExpressionValidationSources(p)
	}
	return c.sources.get(apexFilesKey(p)+"\x01"+p.Root+"\x01"+p.Namespace, func() (*expressionValidationSources, error) {
		return loadExpressionValidationSources(p)
	})
}

func (c *loadCache) rejectedControllers(p project.Project, compilation *ControllerCompilation) (map[string]bool, error) {
	if c == nil {
		return rejectedVisualforceControllersWith(p, compilation)
	}
	return c.rejected.get(apexFilesKey(p)+"\x01"+p.Root, func() (map[string]bool, error) {
		return rejectedVisualforceControllersWith(p, compilation)
	})
}

func (c *loadCache) customComponentProject(p project.Project) error {
	if c == nil {
		return validateCustomComponentProjectWith(p, nil)
	}
	_, err := c.components.get(pageFilesKey(p)+"\x01"+apexFilesKey(p), func() (struct{}, error) {
		return struct{}{}, validateCustomComponentProjectWith(p, c)
	})
	return err
}

func (c *loadCache) alternativeRenderingProject(p project.Project) error {
	if c == nil {
		return validateAlternativeRenderingProject(p)
	}
	_, err := c.alternates.get(pageAndEmailFilesKey(p), func() (struct{}, error) {
		return struct{}{}, validateAlternativeRenderingProject(p)
	})
	return err
}

// strictIndex parses every page and component of p with structure validation.
// The parsed slices are shared between loads of the same file set; callers
// copy them into a fresh Index before building lookups.
func (c *loadCache) strictIndex(p project.Project, fallbackAPIVersion string) (parsedIndex, error) {
	if c == nil {
		return parseStrictIndex(p, fallbackAPIVersion)
	}
	return c.parsed.get(pageFilesKey(p)+"\x01"+fallbackAPIVersion, func() (parsedIndex, error) {
		return parseStrictIndex(p, fallbackAPIVersion)
	})
}

func parseStrictIndex(p project.Project, fallbackAPIVersion string) (parsedIndex, error) {
	var parsed parsedIndex
	for _, path := range p.VisualforcePageFiles {
		page, err := parsePageFile(path, fallbackAPIVersion)
		if err != nil {
			return parsedIndex{}, err
		}
		parsed.pages = append(parsed.pages, page)
	}
	for _, path := range p.VisualforceComponentFiles {
		component, err := parseComponentFile(path, fallbackAPIVersion)
		if err != nil {
			return parsedIndex{}, err
		}
		parsed.components = append(parsed.components, component)
	}
	return parsed, nil
}
