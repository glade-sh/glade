package apextest

import (
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/startupcache"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

var disableDiskCache atomic.Bool

func diskCacheEnabled() bool {
	return !disableDiskCache.Load()
}

func tryLoadDiskRuntime(index typesys.Index) (runtimeCacheEntry, bool) {
	if !diskCacheEnabled() {
		return runtimeCacheEntry{}, false
	}
	root := strings.TrimSpace(index.Project.Root)
	if root == "" {
		return runtimeCacheEntry{}, false
	}
	root = filepath.Clean(root)
	entry, err := startupcache.Read(root, startupcache.SubdirTest)
	if err != nil || entry == nil || !startupcache.Fresh(entry, root, startupcache.Version) {
		return runtimeCacheEntry{}, false
	}
	return runtimeCacheEntryFromStartup(*entry), true
}

func persistDiskRuntime(index typesys.Index, entry runtimeCacheEntry) {
	if !diskCacheEnabled() {
		return
	}
	root := strings.TrimSpace(index.Project.Root)
	if root == "" {
		return
	}
	p, err := project.Load(root)
	if err != nil {
		return
	}
	cacheEntry := startupcache.NewEntry(root, p, index, entry.Org, startupcache.CompiledRuntime{
		Methods:   entry.Methods,
		Classes:   entry.Classes,
		Triggers:  entry.Triggers,
		PageNames: entry.PageNames,
	})
	_ = startupcache.Write(&cacheEntry, startupcache.SubdirTest)
}

func runtimeCacheEntryFromStartup(entry startupcache.Entry) runtimeCacheEntry {
	runtime := CompiledProjectRuntime{
		Methods:   entry.Runtime.Methods,
		Classes:   entry.Runtime.Classes,
		Triggers:  entry.Runtime.Triggers,
		PageNames: entry.Runtime.PageNames,
	}
	pageNames := append([]string(nil), runtime.PageNames...)
	baseMachine := vm.New(nil)
	baseMachine.SetTraceEnabled(false)
	registerVisualforcePages(baseMachine, pageNames)
	baseErr := registerBaseRuntime(baseMachine, runtime.Methods, runtime.Classes, runtime.Triggers)
	org := entry.Org
	return runtimeCacheEntry{
		Methods:       runtime.Methods,
		Classes:       runtime.Classes,
		Triggers:      runtime.Triggers,
		TriggerErrors: nil,
		Org:           org,
		Template:      storage.NewRuntimeTemplate(org),
		PageNames:     pageNames,
		BaseMachine:   baseMachine,
		BaseErr:       baseErr,
	}
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

func scanProjectReferencedStandardFieldsFromSource(org *storage.OrgState, source string, childRelationshipLookup map[string]struct{}) standardFieldScanResult {
	result := standardFieldScanResult{
		Inferred:              make(map[string]map[string]storage.Field),
		ChildRelationshipRefs: make(map[string]projectChildRelationshipSourceReference),
		CustomObjects:         make(map[string]struct{}),
		ListCustomSettings:    make(map[string]struct{}),
	}
	for _, ref := range projectChildRelationshipSourceReferences(source) {
		key := strings.ToLower(ref.ParentObject + "\x00" + ref.ChildObject + "\x00" + ref.ChildRelationship)
		result.ChildRelationshipRefs[key] = ref
	}
	scanSource := apexReferenceScanSource(source)
	varTypes := make(map[string]string)
	for _, match := range apexTypedVariablePattern.FindAllStringSubmatch(scanSource, -1) {
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
	recordBoolean := func(objectName, fieldName string) {
		recordProjectReferencedStandardFieldReadOnly(org, result.Inferred, result.CustomObjects, childRelationshipLookup, objectName, fieldName)
	}
	recordNumeric := recordBoolean
	for _, match := range apexVariableFieldBooleanRightLiteralPattern.FindAllStringSubmatchIndex(scanSource, -1) {
		if len(match) != 6 {
			continue
		}
		objectName, ok := varTypes[scanSource[match[2]:match[3]]]
		if !ok {
			continue
		}
		recordBoolean(objectName, scanSource[match[4]:match[5]])
	}
	for _, match := range apexVariableFieldBooleanLeftLiteralPattern.FindAllStringSubmatchIndex(scanSource, -1) {
		if len(match) != 6 {
			continue
		}
		objectName, ok := varTypes[scanSource[match[2]:match[3]]]
		if !ok {
			continue
		}
		recordBoolean(objectName, scanSource[match[4]:match[5]])
	}
	for _, match := range apexVariableFieldBooleanNegationPattern.FindAllStringSubmatchIndex(scanSource, -1) {
		if len(match) != 6 {
			continue
		}
		objectName, ok := varTypes[scanSource[match[2]:match[3]]]
		if !ok {
			continue
		}
		recordBoolean(objectName, scanSource[match[4]:match[5]])
	}
	for _, match := range apexVariableFieldNumericRightLiteralPattern.FindAllStringSubmatchIndex(scanSource, -1) {
		if len(match) != 6 {
			continue
		}
		objectName, ok := varTypes[scanSource[match[2]:match[3]]]
		if !ok {
			continue
		}
		recordNumeric(objectName, scanSource[match[4]:match[5]])
	}
	for _, match := range apexVariableFieldNumericLeftLiteralPattern.FindAllStringSubmatchIndex(scanSource, -1) {
		if len(match) != 6 {
			continue
		}
		objectName, ok := varTypes[scanSource[match[2]:match[3]]]
		if !ok {
			continue
		}
		recordNumeric(objectName, scanSource[match[4]:match[5]])
	}
	for _, match := range apexCustomSettingGetAllPattern.FindAllStringSubmatchIndex(scanSource, -1) {
		if len(match) != 4 {
			continue
		}
		result.ListCustomSettings[scanSource[match[2]:match[3]]] = struct{}{}
	}
	for _, match := range apexSchemaSObjectTypeFieldReferencePattern.FindAllStringSubmatchIndex(scanSource, -1) {
		if len(match) != 6 {
			continue
		}
		recordProjectReferencedStandardFieldReadOnly(org, result.Inferred, result.CustomObjects, childRelationshipLookup, scanSource[match[2]:match[3]], scanSource[match[4]:match[5]])
	}
	for _, match := range apexSObjectTypeFieldReferencePattern.FindAllStringSubmatchIndex(scanSource, -1) {
		if len(match) != 6 {
			continue
		}
		recordProjectReferencedStandardFieldReadOnly(org, result.Inferred, result.CustomObjects, childRelationshipLookup, scanSource[match[2]:match[3]], scanSource[match[4]:match[5]])
	}
	for _, match := range apexNewSObjectLiteralPattern.FindAllStringSubmatchIndex(scanSource, -1) {
		recordSObjectLiteralLookupFieldsReadOnly(org, result.Inferred, childRelationshipLookup, scanSource, match, varTypes)
	}
	for _, match := range apexSObjectLiteralPattern.FindAllStringSubmatchIndex(scanSource, -1) {
		recordSObjectLiteralLookupFieldsReadOnly(org, result.Inferred, childRelationshipLookup, scanSource, match, varTypes)
	}
	for _, match := range apexNewSObjectLiteralPattern.FindAllStringSubmatchIndex(scanSource, -1) {
		recordSObjectLiteralFieldsReadOnly(org, result.Inferred, childRelationshipLookup, scanSource, match)
	}
	for _, match := range apexSObjectLiteralPattern.FindAllStringSubmatchIndex(scanSource, -1) {
		recordSObjectLiteralFieldsReadOnly(org, result.Inferred, childRelationshipLookup, scanSource, match)
	}
	for _, match := range apexStaticFieldReferencePattern.FindAllStringSubmatchIndex(scanSource, -1) {
		if len(match) != 6 || apexMemberReferenceIsCall(scanSource, match[1]) {
			continue
		}
		recordProjectReferencedStandardFieldReadOnly(org, result.Inferred, result.CustomObjects, childRelationshipLookup, scanSource[match[2]:match[3]], scanSource[match[4]:match[5]])
	}
	for _, match := range apexVariableFieldReferencePattern.FindAllStringSubmatchIndex(scanSource, -1) {
		if len(match) != 6 || apexMemberReferenceIsCall(scanSource, match[1]) {
			continue
		}
		objectName, ok := varTypes[scanSource[match[2]:match[3]]]
		if !ok {
			continue
		}
		recordProjectReferencedStandardFieldReadOnly(org, result.Inferred, result.CustomObjects, childRelationshipLookup, objectName, scanSource[match[4]:match[5]])
	}
	return result
}

func recordProjectReferencedStandardFieldReadOnly(org *storage.OrgState, inferred map[string]map[string]storage.Field, customObjects map[string]struct{}, childRelationshipLookup map[string]struct{}, objectName, fieldName string) {
	if strings.EqualFold(fieldName, "SObjectType") || strings.EqualFold(fieldName, "Fields") {
		return
	}
	state, ok := org.Objects[objectName]
	if !ok {
		if !isCustomDataObjectKey(objectName) {
			return
		}
		if customObjects != nil {
			customObjects[objectName] = struct{}{}
		}
		if inferred[objectName] == nil {
			inferred[objectName] = make(map[string]storage.Field)
		}
		if _, _, ok := projectReferencedInferredField(inferred[objectName], fieldName); !ok {
			inferred[objectName][fieldName] = inferredReferencedField(fieldName)
		}
		return
	}
	if _, ok := storage.ResolveFieldName(state.Definition, org.Namespace, fieldName); ok {
		return
	}
	if parentRelationshipKnown(state.Definition, fieldName) {
		return
	}
	if projectReferencedNameIsChildRelationship(*org, objectName, fieldName, childRelationshipLookup) {
		return
	}
	if inferred[objectName] == nil {
		inferred[objectName] = make(map[string]storage.Field)
	}
	if _, _, ok := projectReferencedInferredField(inferred[objectName], fieldName); ok {
		return
	}
	inferred[objectName][fieldName] = inferredReferencedField(fieldName)
}

func recordSObjectLiteralFieldsReadOnly(org *storage.OrgState, inferred map[string]map[string]storage.Field, childRelationshipLookup map[string]struct{}, scanSource string, match []int) {
	if len(match) != 6 {
		return
	}
	objectName := scanSource[match[2]:match[3]]
	if _, ok := org.Objects[objectName]; !ok {
		return
	}
	body := scanSource[match[4]:match[5]]
	for _, argMatch := range apexNamedArgumentPattern.FindAllStringSubmatchIndex(body, -1) {
		if len(argMatch) != 4 {
			continue
		}
		recordProjectReferencedStandardFieldReadOnly(org, inferred, nil, childRelationshipLookup, objectName, body[argMatch[2]:argMatch[3]])
	}
}

func recordSObjectLiteralLookupFieldsReadOnly(org *storage.OrgState, inferred map[string]map[string]storage.Field, childRelationshipLookup map[string]struct{}, scanSource string, match []int, varTypes map[string]string) {
	if len(match) != 6 {
		return
	}
	objectName := scanSource[match[2]:match[3]]
	if _, ok := org.Objects[objectName]; !ok {
		return
	}
	body := scanSource[match[4]:match[5]]
	for _, argMatch := range apexNamedArgumentSObjectIDPattern.FindAllStringSubmatchIndex(body, -1) {
		if len(argMatch) != 6 {
			continue
		}
		parentObjectName, ok := varTypes[body[argMatch[4]:argMatch[5]]]
		if !ok {
			continue
		}
		recordProjectReferencedLookupFieldReadOnly(org, inferred, childRelationshipLookup, objectName, body[argMatch[2]:argMatch[3]], parentObjectName)
	}
}

func recordProjectReferencedLookupFieldReadOnly(org *storage.OrgState, inferred map[string]map[string]storage.Field, childRelationshipLookup map[string]struct{}, objectName, fieldName, parentObjectName string) {
	if strings.EqualFold(fieldName, "SObjectType") || strings.EqualFold(fieldName, "Fields") {
		return
	}
	canonicalParentObject, ok := storage.ResolveObjectName(*org, parentObjectName)
	if !ok {
		return
	}
	state, ok := org.Objects[objectName]
	if !ok {
		return
	}
	if _, ok := storage.ResolveFieldName(state.Definition, org.Namespace, fieldName); ok {
		return
	}
	if projectReferencedNameIsChildRelationship(*org, objectName, fieldName, childRelationshipLookup) {
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
	if inferred[objectName] == nil {
		inferred[objectName] = make(map[string]storage.Field)
	}
	if _, _, ok := projectReferencedInferredField(inferred[objectName], fieldName); ok {
		return
	}
	inferred[objectName][fieldName] = field
}

func parallelScanProjectReferencedStandardFields(org *storage.OrgState, index typesys.Index, cache sourceCache, childRelationshipLookup map[string]struct{}) standardFieldScanResult {
	files := projectReferencedApexFiles(index)
	if len(files) == 0 {
		return standardFieldScanResult{}
	}
	if childRelationshipLookup == nil {
		childRelationshipLookup = projectReferencedChildRelationshipLookup(*org)
	}
	workers := compileWorkers(len(files))
	if workers <= 1 {
		var merged standardFieldScanResult
		for _, file := range files {
			source, err := cache.read(file)
			if err != nil {
				continue
			}
			merged = mergeStandardFieldScanResults([]standardFieldScanResult{merged, scanProjectReferencedStandardFieldsFromSource(org, source, childRelationshipLookup)})
		}
		return merged
	}
	jobs := make(chan string)
	results := make(chan standardFieldScanResult, len(files))
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for file := range jobs {
				source, err := cache.read(file)
				if err != nil {
					continue
				}
				results <- scanProjectReferencedStandardFieldsFromSource(org, source, childRelationshipLookup)
			}
		}()
	}
	for _, file := range files {
		jobs <- file
	}
	close(jobs)
	wg.Wait()
	close(results)
	collected := make([]standardFieldScanResult, 0, len(files))
	for result := range results {
		collected = append(collected, result)
	}
	return mergeStandardFieldScanResults(collected)
}

func projectReferencedApexFiles(index typesys.Index) []string {
	seen := make(map[string]bool)
	var files []string
	for _, typ := range index.Types {
		if typ.File == "" || typ.Dependency || seen[typ.File] {
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
	features := projectReferencedOrgShapeFeatures(scan.Inferred)
	if len(features) > 0 {
		storage.ApplyOrgShape(org, features)
	}
	applyReferencedStandardFieldSet(org, scan.Inferred, childRelationshipLookup)
}
