package apextest

import (
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/glade-sh/glade/internal/startupcache"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

var disableDiskCache atomic.Bool

const testRuntimeCacheABI = "apextest-runtime-v57"

func DisableDiskCacheForTesting() func() {
	wasDisabled := disableDiskCache.Load()
	disableDiskCache.Store(true)
	return func() {
		disableDiskCache.Store(wasDisabled)
	}
}

// EnableDiskCacheForTesting enables the disk cache until the returned restore
// function is called. It lets integration tests exercise cache-enabled paths
// when their package test harness disables disk I/O by default.
func EnableDiskCacheForTesting() func() {
	wasDisabled := disableDiskCache.Load()
	disableDiskCache.Store(false)
	return func() {
		disableDiskCache.Store(wasDisabled)
	}
}

func diskCacheEnabled() bool {
	return !disableDiskCache.Load()
}

func tryLoadDiskRuntime(index typesys.Index) (runtimeCacheEntry, bool) {
	key := runtimeKey(index)
	return tryLoadDiskRuntimeWithSourceDigests(index, nil, key)
}

func tryLoadDiskRuntimeWithSourceDigests(index typesys.Index, digests *typesys.SourceDigestSet, key runtimeCacheKey) (runtimeCacheEntry, bool) {
	if !diskCacheEnabled() {
		return runtimeCacheEntry{}, false
	}
	input, err := validatedDiskRuntimeInput(index, digests)
	if err != nil {
		return runtimeCacheEntry{}, false
	}
	return tryLoadDiskRuntimeWithValidatedInput(index, key, input)
}

func tryLoadDiskRuntimeWithValidatedInput(index typesys.Index, key runtimeCacheKey, input *startupcache.ValidatedInput) (runtimeCacheEntry, bool) {
	if !diskCacheEnabled() {
		return runtimeCacheEntry{}, false
	}
	root := strings.TrimSpace(index.Project.Root)
	if root == "" {
		return runtimeCacheEntry{}, false
	}
	root = filepath.Clean(root)
	entry, err := startupcache.ReadFreshRuntimeWithValidatedInput(root, startupcache.SubdirTest, startupcache.Version, testRuntimeCacheABI, string(key), input)
	if err != nil || entry == nil {
		return runtimeCacheEntry{}, false
	}
	return runtimeCacheEntryFromStartup(*entry)
}

func tryLoadDiskRuntimeWithPerf(index typesys.Index, digests *typesys.SourceDigestSet, key runtimeCacheKey, counters *runPerfCounters) (runtimeCacheEntry, bool) {
	if !diskCacheEnabled() {
		return runtimeCacheEntry{}, false
	}
	input, err := validatedDiskRuntimeInput(index, digests)
	if err != nil {
		return runtimeCacheEntry{}, false
	}
	return tryLoadDiskRuntimeWithPerfValidatedInput(index, key, input, counters)
}

func tryLoadDiskRuntimeWithPerfValidatedInput(index typesys.Index, key runtimeCacheKey, input *startupcache.ValidatedInput, counters *runPerfCounters) (runtimeCacheEntry, bool) {
	if !diskCacheEnabled() {
		return runtimeCacheEntry{}, false
	}
	root := strings.TrimSpace(index.Project.Root)
	if root == "" {
		return runtimeCacheEntry{}, false
	}
	root = filepath.Clean(root)
	entry, stats, err := startupcache.ReadFreshRuntimeWithValidatedInputAndStats(root, startupcache.SubdirTest, startupcache.Version, testRuntimeCacheABI, string(key), input)
	counters.phases.cacheValidateNS.Add(stats.ValidationNS)
	counters.phases.cacheDecodeNS.Add(stats.DecodeNS)
	if err != nil || entry == nil {
		return runtimeCacheEntry{}, false
	}
	return runtimeCacheEntryFromStartupWithPerf(*entry, counters)
}

func persistDiskRuntime(index typesys.Index, digests *typesys.SourceDigestSet, key runtimeCacheKey, org storage.OrgState, entry runtimeCacheEntry) {
	if !diskCacheEnabled() {
		return
	}
	input, err := validatedDiskRuntimeInput(index, digests)
	if err != nil {
		return
	}
	persistDiskRuntimeWithValidatedInput(index, key, org, entry, input)
}

func persistDiskRuntimeWithValidatedInput(index typesys.Index, key runtimeCacheKey, org storage.OrgState, entry runtimeCacheEntry, input *startupcache.ValidatedInput) {
	if !diskCacheEnabled() {
		return
	}
	root := strings.TrimSpace(index.Project.Root)
	if root == "" {
		return
	}
	canonicalRoot, err := filepath.Abs(root)
	if err != nil || input == nil || filepath.Clean(canonicalRoot) != input.ProjectRoot() {
		return
	}
	cacheEntry, err := startupcache.NewEntryWithValidatedInput(input, org, startupcache.CompiledRuntime{
		Methods:   entry.Methods,
		Classes:   PersistableClasses(entry.Classes),
		Triggers:  entry.Triggers,
		PageNames: entry.PageNames,
	})
	if err != nil {
		return
	}
	cacheEntry.RuntimeABI = testRuntimeCacheABI
	cacheEntry.RuntimeKey = string(key)
	_ = startupcache.Write(&cacheEntry, startupcache.SubdirTest)
}

func persistDiskRuntimeWithPerf(index typesys.Index, digests *typesys.SourceDigestSet, key runtimeCacheKey, org storage.OrgState, entry runtimeCacheEntry, counters *runPerfCounters) {
	if !diskCacheEnabled() {
		return
	}
	input, err := validatedDiskRuntimeInput(index, digests)
	if err != nil {
		return
	}
	persistDiskRuntimeWithPerfValidatedInput(index, key, org, entry, input, counters)
}

func persistDiskRuntimeWithPerfValidatedInput(index typesys.Index, key runtimeCacheKey, org storage.OrgState, entry runtimeCacheEntry, input *startupcache.ValidatedInput, counters *runPerfCounters) {
	if !diskCacheEnabled() {
		return
	}
	root := strings.TrimSpace(index.Project.Root)
	if root == "" {
		return
	}
	canonicalRoot, err := filepath.Abs(root)
	if err != nil || input == nil || filepath.Clean(canonicalRoot) != input.ProjectRoot() {
		return
	}
	cacheEntry, err := startupcache.NewEntryWithValidatedInput(input, org, startupcache.CompiledRuntime{
		Methods:   entry.Methods,
		Classes:   PersistableClasses(entry.Classes),
		Triggers:  entry.Triggers,
		PageNames: entry.PageNames,
	})
	if err != nil {
		return
	}
	cacheEntry.RuntimeABI = testRuntimeCacheABI
	cacheEntry.RuntimeKey = string(key)
	stats, _ := startupcache.WriteWithStats(&cacheEntry, startupcache.SubdirTest)
	counters.phases.cacheEncodeNS.Add(stats.EncodeNS)
}

func validatedDiskRuntimeInput(index typesys.Index, digests *typesys.SourceDigestSet) (*startupcache.ValidatedInput, error) {
	return validatedDiskRuntimeInputWithLookup(index, digests, nil)
}

func validatedDiskRuntimeInputWithLookup(index typesys.Index, digests *typesys.SourceDigestSet, lookup *runtimeDigestLookup) (*startupcache.ValidatedInput, error) {
	root := strings.TrimSpace(index.Project.Root)
	if root == "" {
		return nil, nil
	}
	if lookup == nil {
		return startupcache.ValidateInputWithSourceDigests(filepath.Clean(root), digests)
	}
	return startupcache.ValidateInputWithSourceDigestsAndScopedLookup(filepath.Clean(root), digests, lookup.eligible, lookup.current)
}

func validatedDiskRuntimeInputForPublication(index typesys.Index, digests *typesys.SourceDigestSet, lookup *startupcache.ValidatedInput, digestLookups ...*runtimeDigestLookup) (*startupcache.ValidatedInput, bool) {
	if lookup == nil {
		return nil, false
	}
	var digestLookup *runtimeDigestLookup
	if len(digestLookups) != 0 {
		digestLookup = digestLookups[0]
	}
	current, err := validatedDiskRuntimeInputWithLookup(index, digests, digestLookup)
	if err != nil || current == nil || current.Digest() != lookup.Digest() {
		return nil, false
	}
	return current, true
}

func runtimeCacheEntryFromStartup(entry startupcache.Entry) (runtimeCacheEntry, bool) {
	runtime := CompiledProjectRuntime{
		Methods:   entry.Runtime.Methods,
		Classes:   RestoreLazyClasses(entry.Runtime.Classes),
		Triggers:  entry.Runtime.Triggers,
		PageNames: entry.Runtime.PageNames,
	}
	pageNames := append([]string(nil), runtime.PageNames...)
	baseMachine := vm.New(nil)
	baseMachine.SetTraceEnabled(false)
	registerVisualforcePages(baseMachine, pageNames)
	baseErr := registerBaseRuntime(baseMachine, runtime.Methods, runtime.Classes, runtime.Triggers)
	org := entry.Org
	return validateRestoredRuntimeEntry(runtimeCacheEntry{
		Methods:       runtime.Methods,
		Classes:       runtime.Classes,
		Triggers:      runtime.Triggers,
		TriggerErrors: nil,
		PageNames:     pageNames,
		BaseErr:       baseErr,
		restored:      vm.NewRestoredRuntimeTemplate(org, baseMachine),
	})
}

func runtimeCacheEntryFromStartupWithPerf(entry startupcache.Entry, counters *runPerfCounters) (runtimeCacheEntry, bool) {
	projectStarted := time.Now()
	runtime := CompiledProjectRuntime{
		Methods:   entry.Runtime.Methods,
		Classes:   RestoreLazyClasses(entry.Runtime.Classes),
		Triggers:  entry.Runtime.Triggers,
		PageNames: entry.Runtime.PageNames,
	}
	pageNames := append([]string(nil), runtime.PageNames...)
	baseMachine := vm.New(nil)
	baseMachine.SetTraceEnabled(false)
	registerVisualforcePages(baseMachine, pageNames)
	baseErr := registerBaseRuntime(baseMachine, runtime.Methods, runtime.Classes, runtime.Triggers)
	counters.recordObservedPhaseDuration(&counters.phases.projectCompileNS, time.Since(projectStarted))

	orgStarted := time.Now()
	org := entry.Org
	restored := vm.NewRestoredRuntimeTemplate(org, baseMachine)
	counters.recordObservedPhaseDuration(&counters.phases.orgBuildNS, time.Since(orgStarted))
	return validateRestoredRuntimeEntry(runtimeCacheEntry{
		Methods:       runtime.Methods,
		Classes:       runtime.Classes,
		Triggers:      runtime.Triggers,
		TriggerErrors: nil,
		PageNames:     pageNames,
		BaseErr:       baseErr,
		restored:      restored,
	})
}

func validateRestoredRuntimeEntry(entry runtimeCacheEntry) (runtimeCacheEntry, bool) {
	// Disk payloads are decoded and rebuilt outside the in-memory compiler
	// boundary. Validate their complete structure exactly once before they can
	// be published for narrow runner projections. The marker is not serialized.
	if !validateRuntimeCacheEntryStructure(entry) {
		return runtimeCacheEntry{}, false
	}
	entry.executionProjectionValidated = true
	return entry, true
}

type standardFieldScanResult struct {
	Inferred              map[string]map[string]storage.Field
	ChildRelationshipRefs map[string]projectChildRelationshipSourceReference
	CustomObjects         map[string]struct{}
	ListCustomSettings    map[string]struct{}
}

func mergeStandardFieldScanResults(results []standardFieldScanResult) standardFieldScanResult {
	out := standardFieldScanResult{
		Inferred:              make(map[string]map[string]storage.Field),
		ChildRelationshipRefs: make(map[string]projectChildRelationshipSourceReference),
		CustomObjects:         make(map[string]struct{}),
		ListCustomSettings:    make(map[string]struct{}),
	}
	for _, result := range results {
		for objectName, fields := range result.Inferred {
			if out.Inferred[objectName] == nil {
				out.Inferred[objectName] = make(map[string]storage.Field)
			}
			for fieldName, field := range fields {
				if _, ok := out.Inferred[objectName][fieldName]; !ok {
					out.Inferred[objectName][fieldName] = field
				}
			}
		}
		for key, ref := range result.ChildRelationshipRefs {
			out.ChildRelationshipRefs[key] = ref
		}
		for name := range result.CustomObjects {
			out.CustomObjects[name] = struct{}{}
		}
		for name := range result.ListCustomSettings {
			out.ListCustomSettings[name] = struct{}{}
		}
	}
	return out
}

func scanProjectReferencedStandardFieldsFromSource(org *storage.OrgState, source string, childRelationshipLookup *childRelationshipLookup) standardFieldScanResult {
	result := standardFieldScanResult{
		Inferred:              make(map[string]map[string]storage.Field),
		ChildRelationshipRefs: make(map[string]projectChildRelationshipSourceReference),
		CustomObjects:         make(map[string]struct{}),
		ListCustomSettings:    make(map[string]struct{}),
	}
	scan := standardFieldScan{
		org:           org,
		relationships: childRelationshipLookup,
		inferred:      result.Inferred,
		customObjects: result.CustomObjects,
	}
	matches := standardFieldMatchMemo.get(source, standardFieldMatchesFromSource)
	for _, ref := range matches.childRefs {
		key := strings.ToLower(ref.ParentObject + "\x00" + ref.ChildObject + "\x00" + ref.ChildRelationship)
		result.ChildRelationshipRefs[key] = ref
	}
	scanSource := matches.scanSource
	varTypes := make(map[string]string)
	for _, match := range matches.typed {
		if len(match) != 3 {
			continue
		}
		if _, ok := org.Objects[match[1]]; !ok && isCustomDataObjectKey(match[1]) {
			result.CustomObjects[match[1]] = struct{}{}
		}
		if _, ok := org.Objects[match[1]]; ok || isCustomDataObjectKey(match[1]) {
			varTypes[match[2]] = match[1]
		}
	}
	for _, match := range matches.booleanRight {
		if len(match) != 6 {
			continue
		}
		objectName, ok := varTypes[scanSource[match[2]:match[3]]]
		if !ok {
			continue
		}
		scan.recordStandardField(objectName, scanSource[match[4]:match[5]], true)
	}
	for _, match := range matches.booleanLeft {
		if len(match) != 6 {
			continue
		}
		objectName, ok := varTypes[scanSource[match[2]:match[3]]]
		if !ok {
			continue
		}
		scan.recordStandardField(objectName, scanSource[match[4]:match[5]], true)
	}
	for _, match := range matches.booleanNot {
		if len(match) != 6 {
			continue
		}
		objectName, ok := varTypes[scanSource[match[2]:match[3]]]
		if !ok {
			continue
		}
		scan.recordStandardField(objectName, scanSource[match[4]:match[5]], true)
	}
	for _, match := range matches.numericRight {
		if len(match) != 6 {
			continue
		}
		objectName, ok := varTypes[scanSource[match[2]:match[3]]]
		if !ok {
			continue
		}
		scan.recordStandardField(objectName, scanSource[match[4]:match[5]], true)
	}
	for _, match := range matches.numericLeft {
		if len(match) != 6 {
			continue
		}
		objectName, ok := varTypes[scanSource[match[2]:match[3]]]
		if !ok {
			continue
		}
		scan.recordStandardField(objectName, scanSource[match[4]:match[5]], true)
	}
	for _, match := range matches.getAll {
		if len(match) != 4 {
			continue
		}
		result.ListCustomSettings[scanSource[match[2]:match[3]]] = struct{}{}
	}
	for _, match := range matches.schemaFields {
		if len(match) != 6 || apexMemberReferenceIsCall(scanSource, match[1]) {
			continue
		}
		scan.recordStandardField(scanSource[match[2]:match[3]], scanSource[match[4]:match[5]], true)
	}
	for _, match := range matches.sobjectFields {
		if len(match) != 6 || apexMemberReferenceIsCall(scanSource, match[1]) {
			continue
		}
		scan.recordStandardField(scanSource[match[2]:match[3]], scanSource[match[4]:match[5]], true)
	}
	newLiterals := matches.newLiterals
	literals := matches.literals
	for _, match := range newLiterals {
		scan.recordLiteralLookupFields(scanSource, match, varTypes)
	}
	for _, match := range literals {
		scan.recordLiteralLookupFields(scanSource, match, varTypes)
	}
	for _, match := range newLiterals {
		scan.recordLiteralFields(scanSource, match)
	}
	for _, match := range literals {
		scan.recordLiteralFields(scanSource, match)
	}
	for _, match := range matches.staticFields {
		if len(match) != 6 || apexMemberReferenceIsCall(scanSource, match[1]) {
			continue
		}
		scan.recordStandardField(scanSource[match[2]:match[3]], scanSource[match[4]:match[5]], true)
	}
	for _, match := range matches.variableField {
		if len(match) != 6 || apexMemberReferenceIsCall(scanSource, match[1]) {
			continue
		}
		objectName, ok := varTypes[scanSource[match[2]:match[3]]]
		if !ok {
			continue
		}
		scan.recordStandardField(objectName, scanSource[match[4]:match[5]], true)
	}
	return result
}

type standardFieldScan struct {
	org           *storage.OrgState
	relationships *childRelationshipLookup
	inferred      map[string]map[string]storage.Field
	customObjects map[string]struct{}
	seen          map[standardFieldName]struct{}
}

type standardFieldName struct{ object, field string }

func (s *standardFieldScan) recordInferredField(objectName, fieldName string, field storage.Field) {
	fields := s.inferred[objectName]
	if _, exists := fields[fieldName]; exists {
		return
	}
	// The scanner's field-name captures contain only ASCII characters. Keep
	// the first spelling and shape in the result while deduplicating by case.
	key := standardFieldName{object: objectName, field: strings.ToLower(fieldName)}
	if _, exists := s.seen[key]; exists {
		return
	}
	if s.seen == nil {
		s.seen = make(map[standardFieldName]struct{})
	}
	s.seen[key] = struct{}{}
	if fields == nil {
		fields = make(map[string]storage.Field)
		s.inferred[objectName] = fields
	}
	fields[fieldName] = field
}

func (s *standardFieldScan) recordStandardField(objectName, fieldName string, allowCustomObject bool) {
	if strings.EqualFold(fieldName, "SObjectType") || strings.EqualFold(fieldName, "Fields") {
		return
	}
	state, ok := s.org.Objects[objectName]
	if !ok {
		if !isCustomDataObjectKey(objectName) {
			return
		}
		if allowCustomObject {
			s.customObjects[objectName] = struct{}{}
		}
		s.recordInferredField(objectName, fieldName, inferredReferencedField(fieldName))
		return
	}
	if _, ok := storage.ResolveFieldName(state.Definition, s.org.Namespace, fieldName); ok {
		return
	}
	if parentRelationshipKnown(state.Definition, fieldName) {
		return
	}
	if projectReferencedNameIsChildRelationship(*s.org, objectName, fieldName, s.relationships) {
		return
	}
	s.recordInferredField(objectName, fieldName, inferredReferencedField(fieldName))
}

func (s *standardFieldScan) recordLiteralFields(scanSource string, match []int) {
	if len(match) != 6 {
		return
	}
	objectName := scanSource[match[2]:match[3]]
	if _, ok := s.org.Objects[objectName]; !ok {
		return
	}
	body := scanSource[match[4]:match[5]]
	for _, argMatch := range apexNamedArgumentPattern.FindAllStringSubmatchIndex(body, -1) {
		if len(argMatch) != 4 || !apexNamedArgumentStartsArgument(body, argMatch[0], argMatch[3]) {
			continue
		}
		s.recordStandardField(objectName, body[argMatch[2]:argMatch[3]], false)
	}
}

func (s *standardFieldScan) recordLiteralLookupFields(scanSource string, match []int, varTypes map[string]string) {
	if len(match) != 6 {
		return
	}
	objectName := scanSource[match[2]:match[3]]
	if _, ok := s.org.Objects[objectName]; !ok {
		return
	}
	body := scanSource[match[4]:match[5]]
	for _, argMatch := range apexNamedArgumentSObjectIDPattern.FindAllStringSubmatchIndex(body, -1) {
		if len(argMatch) != 6 || !apexNamedArgumentStartsArgument(body, argMatch[0], argMatch[3]) {
			continue
		}
		parentObjectName, ok := varTypes[body[argMatch[4]:argMatch[5]]]
		if !ok {
			continue
		}
		s.recordLookupField(objectName, body[argMatch[2]:argMatch[3]], parentObjectName)
	}
}

func (s *standardFieldScan) recordLookupField(objectName, fieldName, parentObjectName string) {
	if strings.EqualFold(fieldName, "SObjectType") || strings.EqualFold(fieldName, "Fields") {
		return
	}
	canonicalParentObject, ok := storage.ResolveObjectName(*s.org, parentObjectName)
	if !ok {
		return
	}
	state, ok := s.org.Objects[objectName]
	if !ok {
		return
	}
	if _, ok := storage.ResolveFieldName(state.Definition, s.org.Namespace, fieldName); ok {
		return
	}
	if projectReferencedNameIsChildRelationship(*s.org, objectName, fieldName, s.relationships) {
		return
	}
	field := storage.Field{
		APIName:     fieldName,
		Label:       fieldName,
		Type:        storage.FieldReference,
		DisplayType: string(storage.FieldReference),
		ReferenceTo: []string{canonicalParentObject},
	}
	field.RelationshipName = storage.ParentRelationshipName(field)
	if field.RelationshipName == "" || parentRelationshipKnown(state.Definition, field.RelationshipName) {
		return
	}
	s.recordInferredField(objectName, fieldName, field)
}

func parallelScanProjectReferencedStandardFields(org *storage.OrgState, index typesys.Index, cache *sourceCache, childRelationshipLookup *childRelationshipLookup) standardFieldScanResult {
	files := projectReferencedApexFiles(index)
	if len(files) == 0 {
		return standardFieldScanResult{}
	}
	if childRelationshipLookup == nil {
		childRelationshipLookup = projectReferencedChildRelationshipLookup(*org)
	}
	if compileWorkers(len(files)) <= 1 {
		var merged standardFieldScanResult
		for _, file := range files {
			cache.build.at("scan_standard_fields")
			if source, err := cache.read(file); err == nil {
				merged = mergeStandardFieldScanResults([]standardFieldScanResult{merged, scanProjectReferencedStandardFieldsFromSource(org, source, childRelationshipLookup)})
			}
		}
		return merged
	}
	results := make(chan standardFieldScanResult, len(files))
	runRuntimeBuildJobs(len(files), cache.build, "scan_standard_fields", func(i int) {
		if source, err := cache.read(files[i]); err == nil {
			results <- scanProjectReferencedStandardFieldsFromSource(org, source, childRelationshipLookup)
		}
	})
	close(results)
	collected := make([]standardFieldScanResult, 0, len(files))
	for r := range results {
		collected = append(collected, r)
	}
	return mergeStandardFieldScanResults(collected)
}

func projectReferencedApexFiles(index typesys.Index) []string {
	seen := make(map[string]bool)
	var files []string
	for _, typ := range index.Types {
		if !typ.HasSourceSnapshot() || typ.File == "" || typ.Dependency || seen[typ.File] {
			continue
		}
		seen[typ.File] = true
		files = append(files, typ.File)
	}
	return files
}

func applyStandardFieldScanResult(org *storage.OrgState, scan standardFieldScanResult) {
	for objectName := range scan.CustomObjects {
		ensurePermissionReferencedObject(org, objectName)
	}
	for objectName := range scan.ListCustomSettings {
		recordProjectReferencedListCustomSetting(org, objectName)
	}
	childRelationshipLookup := projectReferencedChildRelationshipLookup(*org)
	applyProjectReferencedSourceChildRelationships(org, scan.Inferred, scan.ChildRelationshipRefs, childRelationshipLookup)
	applyReferencedStandardFieldSet(org, scan.Inferred, childRelationshipLookup)
}
