package apexcov

import (
	"sort"
	"strings"
)

// Recorder collects one test's raw runtime coverage events. It implements
// vm.CoverageSink. A Recorder is used by one VM at a time; it holds source
// positions and names only, and Project.Resolve turns them into lines.
type Recorder struct {
	files map[string]*fileEvents
	// lastName and lastEvents cache the most recent file lookup: statements
	// arrive in long runs from one file.
	lastName   string
	lastEvents *fileEvents
	inits      map[initKey]struct{}
	props      map[string]struct{}
}

type fileEvents struct {
	at   map[point]struct{}
	iter map[point]struct{}
}

type initKey struct {
	class  string
	static bool
}

// NewRecorder returns an empty recorder.
func NewRecorder() *Recorder {
	return &Recorder{files: map[string]*fileEvents{}, inits: map[initKey]struct{}{}, props: map[string]struct{}{}}
}

func (r *Recorder) file(name string) *fileEvents {
	if r.lastEvents != nil && name == r.lastName {
		return r.lastEvents
	}
	events := r.files[name]
	if events == nil {
		events = &fileEvents{at: map[point]struct{}{}, iter: map[point]struct{}{}}
		r.files[name] = events
	}
	r.lastName, r.lastEvents = name, events
	return events
}

// CoverStatement records that the statement, lazy operand, catch clause,
// switch label or declaration starting at line and column ran.
func (r *Recorder) CoverStatement(file string, line, column int) {
	if file == "" || line <= 0 {
		return
	}
	r.file(file).at[point{line, column}] = struct{}{}
}

// CoverIteration records one iteration of the loop statement at line and
// column.
func (r *Recorder) CoverIteration(file string, line, column int) {
	if file == "" || line <= 0 {
		return
	}
	r.file(file).iter[point{line, column}] = struct{}{}
}

// CoverClassInit records a class's static or instance initialization.
func (r *Recorder) CoverClassInit(class string, static bool) {
	if class == "" {
		return
	}
	r.inits[initKey{strings.ToLower(class), static}] = struct{}{}
}

// CoverProperty records an access to a class property.
func (r *Recorder) CoverProperty(class, name string) {
	if class == "" || name == "" {
		return
	}
	r.props[strings.ToLower(class)+"#"+strings.ToLower(name)] = struct{}{}
}

// Merge adds other's events to r.
func (r *Recorder) Merge(other *Recorder) {
	if other == nil {
		return
	}
	for name, events := range other.files {
		mine := r.file(name)
		for p := range events.at {
			mine.at[p] = struct{}{}
		}
		for p := range events.iter {
			mine.iter[p] = struct{}{}
		}
	}
	for k := range other.inits {
		r.inits[k] = struct{}{}
	}
	for k := range other.props {
		r.props[k] = struct{}{}
	}
}

// Component is one class or trigger a run measures.
type Component struct {
	Name string
	Kind string
	File string
	Plan *Plan
}

// Project indexes the components of one run by file and by class name.
type Project struct {
	Components []*Component
	byFile     map[string]*Component
	// byClass maps a lower-case dotted class name, and a nested class's
	// lower-case simple name when it is unique, to its component and the
	// dotted name the component's plan uses.
	byClass map[string]classRef
	// sameNamed lists every component of a dotted class name that two source
	// units declare.
	sameNamed map[string][]classRef
}

type classRef struct {
	component *Component
	dotted    string
}

// NewProject indexes components. Components keep their order.
func NewProject(components []*Component) *Project {
	p := &Project{Components: components, byFile: map[string]*Component{}, byClass: map[string]classRef{}, sameNamed: map[string][]classRef{}}
	simple := map[string][]classRef{}
	for _, c := range components {
		p.byFile[c.File] = c
		if c.Plan == nil || c.Kind != KindClass {
			continue
		}
		for _, dotted := range c.Plan.classes {
			if prior, taken := p.byClass[dotted]; taken {
				if len(p.sameNamed[dotted]) == 0 {
					p.sameNamed[dotted] = []classRef{prior}
				}
				p.sameNamed[dotted] = append(p.sameNamed[dotted], classRef{c, dotted})
			}
			p.byClass[dotted] = classRef{c, dotted}
			if i := strings.LastIndexByte(dotted, '.'); i >= 0 {
				simple[dotted[i+1:]] = append(simple[dotted[i+1:]], classRef{c, dotted})
			}
		}
	}
	for name, refs := range simple {
		if _, taken := p.byClass[name]; !taken && len(refs) == 1 {
			p.byClass[name] = refs[0]
		}
	}
	return p
}

// class resolves a class-init or property event by name. When two source
// units declare the name, the event belongs to the one whose code this test
// ran; with no such evidence it is dropped rather than guessed.
func (p *Project) class(name string, r *Recorder) (classRef, bool) {
	if refs := p.sameNamed[name]; len(refs) > 0 {
		var match []classRef
		for _, ref := range refs {
			if _, ran := r.files[ref.component.File]; ran {
				match = append(match, ref)
			}
		}
		if len(match) == 1 {
			return match[0], true
		}
		return classRef{}, false
	}
	ref, ok := p.byClass[name]
	if ok {
		return ref, true
	}
	// A runtime may qualify the name with a namespace.
	if i := strings.IndexByte(name, '.'); i >= 0 {
		ref, ok = p.byClass[name[i+1:]]
	}
	return ref, ok
}

// Resolve turns a recorder's events into the covered lines of each touched
// component. A component is touched when any event resolves to it, even one
// that covers no line, such as constructing a class with no executable lines.
func (p *Project) Resolve(r *Recorder) map[*Component][]int {
	out := map[*Component]map[int]struct{}{}
	touch := func(c *Component) map[int]struct{} {
		set := out[c]
		if set == nil {
			set = map[int]struct{}{}
			out[c] = set
		}
		return set
	}
	add := func(c *Component, set map[int]struct{}, lines []int) {
		for _, line := range lines {
			if _, ok := c.Plan.exec[line]; ok {
				set[line] = struct{}{}
			}
		}
	}
	if r != nil {
		for name, events := range r.files {
			c := p.byFile[name]
			if c == nil || c.Plan == nil {
				continue
			}
			set := touch(c)
			for at := range events.at {
				if lines, ok := c.Plan.at[at]; ok {
					add(c, set, lines)
					continue
				}
				// A position the plan does not know covers its own line.
				add(c, set, []int{at.line})
			}
			for at := range events.iter {
				add(c, set, c.Plan.iter[at])
			}
		}
		for key := range r.inits {
			ref, ok := p.class(key.class, r)
			if !ok {
				continue
			}
			set := touch(ref.component)
			if key.static {
				add(ref.component, set, ref.component.Plan.staticInit[ref.dotted])
			} else {
				add(ref.component, set, ref.component.Plan.instanceInit[ref.dotted])
			}
		}
		for key := range r.props {
			class, prop, _ := strings.Cut(key, "#")
			ref, ok := p.class(class, r)
			if !ok {
				continue
			}
			add(ref.component, touch(ref.component), ref.component.Plan.props[ref.dotted+"#"+prop])
		}
	}
	result := make(map[*Component][]int, len(out))
	for c, set := range out {
		lines := make([]int, 0, len(set))
		for line := range set {
			lines = append(lines, line)
		}
		sort.Ints(lines)
		result[c] = lines
	}
	return result
}
