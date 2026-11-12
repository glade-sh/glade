package visualforce

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"sync"

	"github.com/glade-sh/glade/internal/project"
)

// The strict loader's project-wide inputs (the controller compile gate and
// the Apex declarations behind expression validation) depend only on the
// project and on the bytes of the files it names. Callers used to recompute
// them on every call: every Visualforce page request, every `exec` with a
// project and every test run paid a whole-project analysis for an unchanged
// project. A project generation holds the computed inputs once.
//
// A generation is identified by its content, never by timestamps: the key
// covers the project value and the digest of every file the project names, so
// any edit, replacement or deletion yields a different key and a stale
// generation cannot be served. A generation is stored only when the key is
// unchanged after the computation, so an edit that races the computation is
// never recorded under the older key. Errors are not retained.

// maxGenerations bounds the retained generations, most recently used first.
const maxGenerations = 3

type projectGeneration struct {
	key string

	gateMu sync.Mutex
	gate   *controllerGate

	sourcesMu sync.Mutex
	sources   *expressionValidationSources
}

var generations struct {
	sync.Mutex
	list []*projectGeneration
}

func resetGenerationCache() {
	generations.Lock()
	generations.list = nil
	generations.Unlock()
}

func generationFor(key string) *projectGeneration {
	generations.Lock()
	defer generations.Unlock()
	for i, g := range generations.list {
		if g.key == key {
			copy(generations.list[1:i+1], generations.list[:i])
			generations.list[0] = g
			return g
		}
	}
	g := &projectGeneration{key: key}
	generations.list = append([]*projectGeneration{g}, generations.list...)
	if len(generations.list) > maxGenerations {
		generations.list = generations.list[:maxGenerations]
	}
	return g
}

// sharedControllerGate returns the generation's controller gate, computing it
// with compile on first use. Concurrent callers of one generation wait for a
// single computation.
func sharedControllerGate(p project.Project, compile func() (controllerGate, error)) (controllerGate, error) {
	key, ok := projectGenerationKey(p)
	if !ok {
		return compile()
	}
	g := generationFor(key)
	g.gateMu.Lock()
	defer g.gateMu.Unlock()
	if g.gate != nil {
		return *g.gate, nil
	}
	if gate, ok := loadPersistedControllerGate(key); ok {
		g.gate = &gate
		return gate, nil
	}
	gate, err := compile()
	if err != nil {
		return controllerGate{}, err
	}
	if after, ok := projectGenerationKey(p); ok && after == key {
		g.gate = &gate
		storePersistedControllerGate(key, gate)
	}
	return gate, nil
}

// sharedExpressionSources is sharedControllerGate for the expression
// validation sources. The returned value is read-only for every caller.
func sharedExpressionSources(p project.Project) (*expressionValidationSources, error) {
	key, ok := projectGenerationKey(p)
	if !ok {
		return loadExpressionValidationSources(p)
	}
	g := generationFor(key)
	g.sourcesMu.Lock()
	defer g.sourcesMu.Unlock()
	if g.sources != nil {
		return g.sources, nil
	}
	sources, err := loadExpressionValidationSources(p)
	if err != nil {
		return nil, err
	}
	if after, ok := projectGenerationKey(p); ok && after == key {
		g.sources = sources
	}
	return sources, nil
}

// projectGenerationKey digests the project value and the content of every file
// it names. Every string reachable from the project that names a regular file
// counts, as does that file's "-meta.xml" companion and an object's
// "sharingReasons" directory, so a file list added to the project later is
// covered without a change here. Unreadable paths contribute their error
// text, so a file that appears or vanishes changes the key. ok is false when
// the project cannot be encoded.
func projectGenerationKey(p project.Project) (string, bool) {
	encoded, err := json.Marshal(p)
	if err != nil {
		return "", false
	}
	h := sha256.New()
	_, _ = h.Write(encoded)
	strs := map[string]struct{}{}
	collectStrings(reflect.ValueOf(p), strs)
	paths := make([]string, 0, len(strs))
	for s := range strs {
		if s != "" {
			paths = append(paths, s)
		}
	}
	sort.Strings(paths)
	dirs := map[string]bool{}
	for _, path := range paths {
		if !hashRegularFile(h, path) {
			continue
		}
		hashRegularFile(h, path+"-meta.xml")
		if dir := filepath.Join(filepath.Dir(path), "sharingReasons"); !dirs[dir] {
			dirs[dir] = true
			hashSharingReasons(h, dir)
		}
	}
	// The compile also reads inputs no string in the project names: sema and
	// typesys probe for sfdx-project.json at the root and above every Apex
	// source (dependencies and shims included, up to the file system root),
	// and schema and typesys read the org shape (scratch definitions, glade
	// config, settings and Person Account evidence). Key the files and the
	// values derived from them.
	markerDirs := map[string]bool{}
	var visit func(project.Project)
	visit = func(q project.Project) {
		markerDirs[q.Root] = true
		for _, path := range q.ApexFiles {
			for dir := filepath.Dir(path); !markerDirs[dir]; dir = filepath.Dir(dir) {
				markerDirs[dir] = true
				if filepath.Dir(dir) == dir {
					break
				}
			}
		}
		_, _ = io.WriteString(h, "\x00shape\x00"+q.Root+"\x00")
		fmt.Fprintf(h, "%q\n", project.OrgShapeFeatures(q.Root))
		records := project.ScratchObjectDefaultRecordTypes(q.Root)
		keys := make([]string, 0, len(records))
		for k := range records {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(h, "%q=%q\n", k, records[k])
		}
		for _, f := range project.OrgShapeInputFiles(q.Root) {
			hashRegularFile(h, f)
		}
		for _, dep := range q.ManagedPackageDependencies {
			if dep.Project != nil {
				visit(*dep.Project)
			}
		}
		for _, shim := range q.PackageShims {
			if shim.Project != nil {
				visit(*shim.Project)
			}
		}
	}
	visit(p)
	fmt.Fprintf(h, "%q %q\n", project.StandardFieldPolicyFeatures(p), project.StaticOnlyShapeFeatures(p))
	for _, f := range project.EffectiveOrgShapeInputFiles(p) {
		hashRegularFile(h, f)
	}
	dirList := make([]string, 0, len(markerDirs))
	for dir := range markerDirs {
		dirList = append(dirList, dir)
	}
	sort.Strings(dirList)
	for _, dir := range dirList {
		hashRegularFile(h, filepath.Join(dir, "sfdx-project.json"))
	}
	return hex.EncodeToString(h.Sum(nil)), true
}

// hashRegularFile folds the file's identity and content into h and reports
// whether path names a regular file. A missing file contributes nothing.
func hashRegularFile(h io.Writer, path string) bool {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return false
	}
	_, _ = io.WriteString(h, "\x00file\x00"+path+"\x00")
	digest, err := fileContentDigest(path)
	if err != nil {
		_, _ = io.WriteString(h, "unreadable: "+err.Error())
		return true
	}
	_, _ = h.Write(digest[:])
	return true
}

func hashSharingReasons(h io.Writer, dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	_, _ = io.WriteString(h, "\x00dir\x00"+dir+"\x00")
	for _, entry := range entries {
		_, _ = io.WriteString(h, entry.Name()+"\x00")
		if !entry.IsDir() {
			hashRegularFile(h, filepath.Join(dir, entry.Name()))
		}
	}
}

func collectStrings(v reflect.Value, out map[string]struct{}) {
	switch v.Kind() {
	case reflect.String:
		out[v.String()] = struct{}{}
	case reflect.Slice, reflect.Array:
		if v.Kind() == reflect.Slice && v.IsNil() {
			return
		}
		for i := range v.Len() {
			collectStrings(v.Index(i), out)
		}
	case reflect.Map:
		iter := v.MapRange()
		for iter.Next() {
			collectStrings(iter.Key(), out)
			collectStrings(iter.Value(), out)
		}
	case reflect.Pointer, reflect.Interface:
		if !v.IsNil() {
			collectStrings(v.Elem(), out)
		}
	case reflect.Struct:
		for i := range v.NumField() {
			collectStrings(v.Field(i), out)
		}
	}
}
