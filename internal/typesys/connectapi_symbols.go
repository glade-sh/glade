package typesys

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/apexversion"
)

// connectAPIShapeArchive is the org-captured ConnectApi shape written by
// scripts/generate-connectapi-symbols.mjs from testdata/generated/connectapi_shape.json.
// It stays compressed until the first lookup that needs it, so a process that
// never names ConnectApi pays no init, data-segment or GC-root cost for the
// 6.7k types.
//
//go:embed connectapi_shape_generated.tsv.gz
var connectAPIShapeArchive []byte

// connectAPIShapeRequested is set once any Apex source, index signature or
// caller names the ConnectApi namespace. It is sticky for the process: the
// standard platform symbol view then carries the captured shape, and the
// stub-only view it replaces is never handed out again.
var connectAPIShapeRequested atomic.Bool

func init() {
	apexast.SetSourceObserver(NoteApexSource)
}

// RequestConnectAPIShape adds the org-captured ConnectApi shape to the
// standard platform symbols. The next StandardPlatformSymbolView rebuilds the
// view and PlatformViewCache holders rebuild from it.
func RequestConnectAPIShape() {
	connectAPIShapeRequested.Store(true)
}

// ConnectAPIShapeRequested reports whether the captured ConnectApi shape is
// part of the standard platform symbols for this process.
func ConnectAPIShapeRequested() bool {
	return connectAPIShapeRequested.Load()
}

// NoteApexSource requests the ConnectApi shape when source names the
// namespace in any letter case. Every Apex source the process parses or
// indexes passes through here, so the request precedes the analysis that
// needs the shape.
func NoteApexSource(source string) {
	if connectAPIShapeRequested.Load() {
		return
	}
	if mentionsConnectAPI(source) {
		connectAPIShapeRequested.Store(true)
	}
}

// NoteIndexSignatures requests the ConnectApi shape when a type signature in
// the index (a superclass, interface, member type or parameter type) names
// the namespace. Dependency artifacts carry signatures without source.
func NoteIndexSignatures(index Index) {
	if connectAPIShapeRequested.Load() {
		return
	}
	for i := range index.Types {
		if typeSymbolNamesConnectAPI(&index.Types[i]) {
			connectAPIShapeRequested.Store(true)
			return
		}
	}
}

func typeSymbolNamesConnectAPI(typ *TypeSymbol) bool {
	if mentionsConnectAPI(typ.Name) || mentionsConnectAPI(typ.Namespace) || mentionsConnectAPI(typ.SuperClass) {
		return true
	}
	for _, name := range typ.Interfaces {
		if mentionsConnectAPI(name) {
			return true
		}
	}
	for i := range typ.Members {
		member := &typ.Members[i]
		if mentionsConnectAPI(member.Type) {
			return true
		}
		for _, param := range member.Parameters {
			if mentionsConnectAPI(param.Type) {
				return true
			}
		}
	}
	return false
}

const connectAPINamespaceWord = "connectapi"

// mentionsConnectAPI reports a case-insensitive "connectapi" anywhere in s.
// The scan folds ASCII letters only; the word has no other characters.
func mentionsConnectAPI(s string) bool {
	for i := 0; i+len(connectAPINamespaceWord) <= len(s); i++ {
		if s[i]|0x20 != 'c' {
			continue
		}
		if asciiFoldHasPrefix(s[i:], connectAPINamespaceWord) {
			return true
		}
	}
	return false
}

// asciiFoldHasPrefix reports whether s starts with the lower-case ASCII
// letters of word, ignoring the case of s.
func asciiFoldHasPrefix(s, word string) bool {
	if len(s) < len(word) {
		return false
	}
	for i := 0; i < len(word); i++ {
		if s[i]|0x20 != word[i] {
			return false
		}
	}
	return true
}

func hasConnectAPIPrefix(name string) bool {
	return asciiFoldHasPrefix(name, connectAPINamespaceWord+".")
}

// connectAPIShape is the decoded archive.
type connectAPIShape struct {
	specs              []StandardSymbolSpec
	superClasses       map[string]string
	memberAvailability map[string]apexversion.Range
	exactAvailability  map[string]apexversion.Range
	outputTypes        map[string]bool
	typeNames          map[string]struct{}
	jsonProfiles       map[string]*ConnectAPIJSON
	specIndex          map[string]int
}

// ConnectAPIFailure is a captured native failure of Type.forName(name).newInstance()
// or JSON.serialize for one ConnectApi type.
type ConnectAPIFailure struct {
	// Phase is "forName" (Type.forName answers null), "newInstance" or "serialize".
	Phase string
	// Exception is the qualified exception type name.
	Exception string
	// Message may contain {T}, the type name.
	Message string
}

// ConnectAPIJSON describes how JSON.serialize writes an output class when it
// differs from the default: every property that was not removed, ascending by
// name, an unassigned property as null.
type ConnectAPIJSON struct {
	// OmitNulls drops properties that are null.
	OmitNulls bool
	// Whole is the JSON text the whole value serializes to (null, []); empty otherwise.
	Whole string
	// Keys is the serialized key order when it is not the default; nil otherwise.
	Keys []string
	// Fail describes a call that throws instead of producing JSON.
	Fail *ConnectAPIFailure
	// Defaults maps a property to the JSON literal written while it is unassigned.
	Defaults map[string]string
	// Constants lists the defaults that an assigned value does not replace.
	Constants map[string]bool
}

var (
	connectAPIShapeOnce  sync.Once
	connectAPIShapeCache *connectAPIShape
)

// connectAPICapturedShape decodes the embedded archive once per process.
func connectAPICapturedShape() *connectAPIShape {
	connectAPIShapeOnce.Do(func() {
		shape, err := decodeConnectAPIShape(connectAPIShapeArchive)
		if err != nil {
			panic(fmt.Sprintf("typesys: embedded ConnectApi shape: %v", err))
		}
		connectAPIShapeCache = shape
	})
	return connectAPIShapeCache
}

// The archive is a tab-separated record stream; scripts/generate-connectapi-symbols.mjs
// documents each record. Fields are substrings of the inflated text, so the
// decoded shape shares one buffer instead of one allocation per name.
const connectAPIShapeFormat = "glade-connectapi-shape\t1"

func decodeConnectAPIShape(archive []byte) (*connectAPIShape, error) {
	reader, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	text := string(data)
	header, rest, ok := strings.Cut(text, "\n")
	if !ok || header != connectAPIShapeFormat {
		return nil, fmt.Errorf("unexpected archive header %q", header)
	}
	shape := &connectAPIShape{
		specs:              make([]StandardSymbolSpec, 0, strings.Count(rest, "\nt\t")+1),
		superClasses:       map[string]string{},
		memberAvailability: map[string]apexversion.Range{},
		exactAvailability:  map[string]apexversion.Range{},
		outputTypes:        map[string]bool{},
		typeNames:          map[string]struct{}{},
		jsonProfiles:       map[string]*ConnectAPIJSON{},
		specIndex:          map[string]int{},
	}
	var current *StandardSymbolSpec
	fields := make([]string, 0, 16)
	for line := 1; rest != ""; line++ {
		var record string
		record, rest, _ = strings.Cut(rest, "\n")
		fields = splitTabs(fields[:0], record)
		if len(fields) == 0 {
			return nil, fmt.Errorf("line %d: empty record", line)
		}
		tag, args := fields[0], fields[1:]
		if tag != "t" && tag != "s" && tag != "r" && tag != "x" && tag != "o" && tag != "j" && tag != "jk" && tag != "jd" && tag != "jw" && tag != "jx" && current == nil {
			return nil, fmt.Errorf("line %d: %q record before any type", line, tag)
		}
		switch tag {
		case "t":
			if len(args) != 2 {
				return nil, fmt.Errorf("line %d: type record has %d fields", line, len(args))
			}
			spec := StandardSymbolSpec{Name: args[0]}
			if strings.ContainsRune(args[1], 'e') {
				spec.Kind = apexast.DeclarationEnum
			}
			if strings.ContainsRune(args[1], 'v') {
				spec.Modifiers = []string{"virtual"}
			}
			spec.ReplaceConstructors = strings.ContainsRune(args[1], 'r')
			shape.specs = append(shape.specs, spec)
			current = &shape.specs[len(shape.specs)-1]
			shape.typeNames[strings.ToLower(spec.Name)] = struct{}{}
			shape.specIndex[strings.ToLower(spec.Name)] = len(shape.specs) - 1
		case "c":
			params, err := connectAPIShapeParameters(args)
			if err != nil {
				return nil, fmt.Errorf("line %d: %v", line, err)
			}
			if params == nil {
				params = []StandardParameterSpec{}
			}
			current.ConstructorSpecs = append(current.ConstructorSpecs, StandardConstructorSpec{Parameters: params})
		case "m":
			if len(args) < 3 {
				return nil, fmt.Errorf("line %d: method record has %d fields", line, len(args))
			}
			params, err := connectAPIShapeParameters(args[3:])
			if err != nil {
				return nil, fmt.Errorf("line %d: %v", line, err)
			}
			method := StandardMethodSpec{Name: args[0], ReturnType: args[1], ParameterSpecs: params, Static: strings.ContainsRune(args[2], 's')}
			if strings.ContainsRune(args[2], 'v') {
				method.Modifiers = []string{"virtual"}
			}
			current.Methods = append(current.Methods, method)
		case "p":
			if len(args) != 3 {
				return nil, fmt.Errorf("line %d: property record has %d fields", line, len(args))
			}
			current.Properties = append(current.Properties, StandardPropertySpec{Name: args[0], Type: args[1], Static: strings.ContainsRune(args[2], 's'), Force: strings.ContainsRune(args[2], 'f')})
		case "s":
			if len(args) != 2 {
				return nil, fmt.Errorf("line %d: superclass record has %d fields", line, len(args))
			}
			shape.superClasses[args[0]] = args[1]
		case "r", "x":
			if len(args) != 3 {
				return nil, fmt.Errorf("line %d: range record has %d fields", line, len(args))
			}
			since, err := strconv.Atoi(args[1])
			if err != nil {
				return nil, fmt.Errorf("line %d: %v", line, err)
			}
			until, err := strconv.Atoi(args[2])
			if err != nil {
				return nil, fmt.Errorf("line %d: %v", line, err)
			}
			if tag == "r" {
				shape.memberAvailability[args[0]] = apexversion.Range{Since: since, Until: until}
			} else {
				shape.exactAvailability[args[0]] = apexversion.Range{Since: since, Until: until}
			}
		case "o":
			if len(args) != 1 {
				return nil, fmt.Errorf("line %d: output record has %d fields", line, len(args))
			}
			shape.outputTypes[args[0]] = true
		case "j":
			if len(args) != 2 {
				return nil, fmt.Errorf("line %d: json record has %d fields", line, len(args))
			}
			shape.jsonProfiles[args[0]] = &ConnectAPIJSON{
				OmitNulls: strings.ContainsRune(args[1], 'n'),
			}
		case "jw":
			profile := shape.jsonProfiles[args[0]]
			if profile == nil || len(args) != 2 {
				return nil, fmt.Errorf("line %d: bad json whole record", line)
			}
			profile.Whole = args[1]
		case "jx":
			profile := shape.jsonProfiles[args[0]]
			if profile == nil || len(args) != 4 {
				return nil, fmt.Errorf("line %d: bad json failure record", line)
			}
			profile.Fail = &ConnectAPIFailure{Phase: args[1], Exception: args[2], Message: args[3]}
		case "jk", "jd":
			profile := shape.jsonProfiles[args[0]]
			if profile == nil {
				return nil, fmt.Errorf("line %d: %q record before its json record", line, tag)
			}
			if tag == "jk" {
				profile.Keys = append([]string(nil), args[1:]...)
				break
			}
			if len(args) != 3 && len(args) != 4 {
				return nil, fmt.Errorf("line %d: json default record has %d fields", line, len(args))
			}
			if profile.Defaults == nil {
				profile.Defaults = map[string]string{}
			}
			profile.Defaults[args[1]] = args[2]
			if len(args) == 4 {
				if profile.Constants == nil {
					profile.Constants = map[string]bool{}
				}
				profile.Constants[args[1]] = true
			}
		default:
			return nil, fmt.Errorf("line %d: unknown record %q", line, tag)
		}
	}
	return shape, nil
}

// splitTabs appends the tab-separated fields of record to out.
func splitTabs(out []string, record string) []string {
	for {
		field, rest, ok := strings.Cut(record, "\t")
		out = append(out, field)
		if !ok {
			return out
		}
		record = rest
	}
}

// connectAPIShapeParameters reads name/type pairs. No pairs decode to nil.
func connectAPIShapeParameters(fields []string) ([]StandardParameterSpec, error) {
	if len(fields)%2 != 0 {
		return nil, fmt.Errorf("parameter fields %q are not name/type pairs", fields)
	}
	if len(fields) == 0 {
		return nil, nil
	}
	out := make([]StandardParameterSpec, 0, len(fields)/2)
	for i := 0; i < len(fields); i += 2 {
		out = append(out, StandardParameterSpec{Name: fields[i], Type: fields[i+1]})
	}
	return out, nil
}

// ConnectAPIMemberAvailability returns the org-captured version range of a
// ConnectApi property or constructor. owner is the qualified type name.
func ConnectAPIMemberAvailability(owner, member string) (apexversion.Range, bool) {
	owner = strings.TrimSpace(owner)
	if !hasConnectAPIPrefix(owner) {
		return apexversion.Range{}, false
	}
	rng, ok := connectAPICapturedShape().memberAvailability[strings.ToLower(owner)+"."+strings.ToLower(strings.TrimSpace(member))]
	return rng, ok
}

// ConnectAPIExactAvailability returns the captured range for one static method
// overload, keyed "apex:connectapi.<type>.<method>(<lower-case parameter types>)".
func ConnectAPIExactAvailability(surfaceID string) (apexversion.Range, bool) {
	surfaceID = strings.TrimSpace(surfaceID)
	if !asciiFoldHasPrefix(surfaceID, "apex:"+connectAPINamespaceWord+".") {
		return apexversion.Range{}, false
	}
	rng, ok := connectAPICapturedShape().exactAvailability[strings.ToLower(surfaceID)]
	return rng, ok
}

// ConnectAPIOutputType reports a captured ConnectApi output (representation)
// class: its String.valueOf starts with buildVersion and JSON.serialize accepts it.
func ConnectAPIOutputType(name string) bool {
	name = strings.TrimSpace(name)
	if !hasConnectAPIPrefix(name) {
		return false
	}
	return connectAPICapturedShape().outputTypes[strings.ToLower(name)]
}

// ConnectAPICapturedType reports a type present in the org-captured ConnectApi shape.
func ConnectAPICapturedType(name string) bool {
	name = strings.TrimSpace(name)
	if !hasConnectAPIPrefix(name) {
		return false
	}
	_, ok := connectAPICapturedShape().typeNames[strings.ToLower(name)]
	return ok
}

// ConnectAPIJSONProfile returns the captured JSON.serialize profile of a
// ConnectApi output class that deviates from the default.
func ConnectAPIJSONProfile(name string) (*ConnectAPIJSON, bool) {
	name = strings.TrimSpace(name)
	if !hasConnectAPIPrefix(name) {
		return nil, false
	}
	profile, ok := connectAPICapturedShape().jsonProfiles[strings.ToLower(name)]
	return profile, ok
}

// ConnectAPIJSONKeys returns the keys JSON.serialize writes for a captured
// ConnectApi class by default: the properties of the class and its captured
// ancestors that were not removed, ascending by name. The second result is
// false when the class is not in the captured shape.
func ConnectAPIJSONKeys(name string) ([]string, bool) {
	name = strings.TrimSpace(name)
	if !hasConnectAPIPrefix(name) {
		return nil, false
	}
	shape := connectAPICapturedShape()
	key := strings.ToLower(name)
	if _, ok := shape.specIndex[key]; !ok {
		return nil, false
	}
	seen := map[string]bool{}
	var keys []string
	for visited := 0; visited < 64; visited++ {
		index, ok := shape.specIndex[key]
		if !ok {
			break
		}
		spec := shape.specs[index]
		for _, prop := range spec.Properties {
			if !prop.Force || prop.Static || seen[strings.ToLower(prop.Name)] {
				continue
			}
			seen[strings.ToLower(prop.Name)] = true
			if rng, ok := shape.memberAvailability[key+"."+strings.ToLower(prop.Name)]; ok && rng.Until > 0 {
				continue
			}
			keys = append(keys, prop.Name)
		}
		parent, ok := shape.superClasses[key]
		if !ok {
			break
		}
		key = strings.ToLower(parent)
	}
	sort.Strings(keys)
	return keys, true
}
