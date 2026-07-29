package sema

import (
	"sort"
	"strings"
	"sync"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

type typeMembers struct {
	name                              string
	shortKey                          string
	namespace                         string
	dependency                        bool
	sobject                           bool
	externalPackageSObject            bool
	partialSObject                    bool
	nestingDepth                      int
	kind                              apexast.DeclarationKind
	superClass                        string
	interfaces                        []string
	modifiers                         []string
	methods                           map[string][]typesys.MemberSymbol
	constructors                      []typesys.MemberSymbol
	fields                            map[string]typesys.MemberSymbol
	syntheticChildRelationshipAliases map[string]struct{}
	syntheticStandardSObject          bool
	standardSObjectFieldsLoaded       bool
}

type semaTypeMemberModel struct {
	members         map[string]typeMembers
	shortCandidates map[string][]string
	platform        *semaLazyPlatformTypeMemberModel
}

type semaLazyPlatformTypeMemberModel struct {
	symbols        []typesys.TypeSymbol
	symbolsByKey   map[string]*typesys.TypeSymbol
	candidates     map[string][]string
	lookups        sync.Map
	fieldNamesOnce sync.Once
	fieldNames     map[string]struct{}
}

type semaPlatformTypeMemberLookup struct {
	members typeMembers
	ok      bool
}

func (m *semaTypeMemberModel) lookup(key string) (typeMembers, bool) {
	if m == nil {
		return typeMembers{}, false
	}
	if members, ok := m.members[key]; ok {
		return members, true
	}
	if m.platform != nil {
		return m.platform.lookup(key)
	}
	return typeMembers{}, false
}

func (m *semaTypeMemberModel) candidateKeys(short string) []string {
	if m == nil {
		return nil
	}
	if candidates := m.shortCandidates[short]; len(candidates) > 0 {
		return candidates
	}
	if m.platform != nil {
		return m.platform.candidateKeys(short)
	}
	return nil
}

func (m *semaTypeMemberModel) hasField(key string) bool {
	if m == nil || key == "" {
		return false
	}
	for _, members := range m.members {
		if _, ok := members.fields[key]; ok {
			return true
		}
	}
	return m.platform != nil && m.platform.hasField(key)
}

type semaTypeMemberState struct {
	base     *semaTypeMemberModel
	platform *semaTypeMemberModel
}

type semaTypeMemberView struct {
	state    *semaTypeMemberState
	current  map[string]typeMembers
	hydrated map[string]typeMembers
}

func newSemaTypeMemberState(base *semaTypeMemberModel) *semaTypeMemberState {
	return newSemaTypeMemberStateWithPlatform(base, semaPlatformTypeMemberModel())
}

func newSemaTypeMemberStateWithPlatform(base, platform *semaTypeMemberModel) *semaTypeMemberState {
	return &semaTypeMemberState{
		base:     base,
		platform: platform,
	}
}

func (s *semaTypeMemberState) view() *semaTypeMemberView {
	return &semaTypeMemberView{
		state:    s,
		hydrated: make(map[string]typeMembers),
	}
}

func (v *semaTypeMemberView) lookup(key string) (typeMembers, bool) {
	if v == nil || v.state == nil {
		return typeMembers{}, false
	}
	if members, ok := v.current[key]; ok {
		return members, true
	}
	if members, ok := v.hydrated[key]; ok {
		return members, true
	}
	if v.state.base != nil {
		if members, ok := v.state.base.lookup(key); ok {
			return members, true
		}
	}
	if v.state.platform != nil {
		members, ok := v.state.platform.lookup(key)
		if ok {
			v.hydrated[key] = members
		}
		return members, ok
	}
	return typeMembers{}, false
}

func (v *semaTypeMemberView) get(key string) typeMembers {
	members, _ := v.lookup(key)
	return members
}

func (v *semaTypeMemberView) storeHydrated(key string, members typeMembers) {
	if v == nil || v.state == nil || key == "" {
		return
	}
	v.hydrated[key] = members
}

func (v *semaTypeMemberView) shortCandidateKeys(short string) []string {
	if v == nil || v.state == nil {
		return nil
	}
	var base, platform []string
	if v.state.base != nil {
		base = v.state.base.candidateKeys(short)
	}
	if v.state.platform != nil {
		platform = v.state.platform.candidateKeys(short)
	}
	if len(base) == 0 {
		return platform
	}
	if len(platform) == 0 {
		return base
	}
	out := make([]string, 0, len(base)+len(platform))
	out = append(out, base...)
	seen := make(map[string]bool, len(base))
	for _, key := range base {
		seen[key] = true
	}
	for _, key := range platform {
		if !seen[key] {
			out = append(out, key)
		}
	}
	return out
}

var semaPlatformTypeMemberModelCache struct {
	once  sync.Once
	model *semaTypeMemberModel
}

func semaPlatformTypeMemberModel() *semaTypeMemberModel {
	semaPlatformTypeMemberModelCache.once.Do(func() {
		semaPlatformTypeMemberModelCache.model = buildSemaPlatformTypeMemberModel()
	})
	return semaPlatformTypeMemberModelCache.model
}

func buildSemaPlatformTypeMemberModel() *semaTypeMemberModel {
	symbols := typesys.StandardPlatformSymbolView()
	return &semaTypeMemberModel{
		members:  map[string]typeMembers{},
		platform: newSemaLazyPlatformTypeMemberModel(symbols),
	}
}

func newSemaLazyPlatformTypeMemberModel(symbols []typesys.TypeSymbol) *semaLazyPlatformTypeMemberModel {
	// The typesys view is process-immutable. Index names once, then materialize
	// member maps only for symbols an analysis actually resolves.
	model := &semaLazyPlatformTypeMemberModel{
		symbols:      symbols,
		symbolsByKey: make(map[string]*typesys.TypeSymbol, len(symbols)*2),
		candidates:   make(map[string][]string),
	}
	shortAliases := make(map[string]*typesys.TypeSymbol)
	ambiguousShortAliases := make(map[string]bool)
	for i := range symbols {
		symbol := &symbols[i]
		localKey := normalizeName(symbol.Name)
		qualifiedKey := localKey
		if symbol.Namespace != "" {
			qualifiedKey = normalizeName(symbol.Namespace) + "." + localKey
		}
		if _, exists := model.symbolsByKey[localKey]; !exists {
			model.symbolsByKey[localKey] = symbol
		}
		model.symbolsByKey[qualifiedKey] = symbol
		short := semaShortTypeKeyFromNormalizedKey(localKey)
		if short == "" {
			continue
		}
		if short != localKey {
			if existing, exists := shortAliases[short]; !exists {
				shortAliases[short] = symbol
			} else if existing != symbol {
				ambiguousShortAliases[short] = true
			}
			model.candidates[short] = appendUniqueSemaCandidate(model.candidates[short], short)
		}
		model.candidates[short] = appendUniqueSemaCandidate(model.candidates[short], localKey)
		model.candidates[short] = appendUniqueSemaCandidate(model.candidates[short], qualifiedKey)
	}
	for short, symbol := range shortAliases {
		if !ambiguousShortAliases[short] {
			if _, exists := model.symbolsByKey[short]; !exists {
				model.symbolsByKey[short] = symbol
			}
		}
	}
	for short := range ambiguousShortAliases {
		if _, exists := model.symbolsByKey[short]; exists {
			continue
		}
		candidates := model.candidates[short]
		filtered := candidates[:0]
		for _, candidate := range candidates {
			if candidate != short {
				filtered = append(filtered, candidate)
			}
		}
		model.candidates[short] = filtered
	}
	return model
}

func appendUniqueSemaCandidate(candidates []string, key string) []string {
	for _, existing := range candidates {
		if existing == key {
			return candidates
		}
	}
	return append(candidates, key)
}

func (m *semaLazyPlatformTypeMemberModel) lookup(key string) (typeMembers, bool) {
	if m == nil || key == "" {
		return typeMembers{}, false
	}
	if cached, ok := m.lookups.Load(key); ok {
		result := cached.(semaPlatformTypeMemberLookup)
		return semaCloneTypeMembers(result.members), result.ok
	}
	result := semaPlatformTypeMemberLookup{}
	if objectName, ok := semaStandardSObjectNameForKey(key); ok {
		result.members = semaStandardSObjectPlaceholder(objectName)
		result.ok = true
	} else if symbol, ok := m.symbolsByKey[key]; ok {
		result.members = semaTypeMembersFromPlatformSymbol(*symbol)
		result.ok = true
	} else {
		return typeMembers{}, false
	}
	actual, _ := m.lookups.LoadOrStore(key, result)
	result = actual.(semaPlatformTypeMemberLookup)
	return semaCloneTypeMembers(result.members), result.ok
}

func (m *semaLazyPlatformTypeMemberModel) candidateKeys(short string) []string {
	if m == nil || short == "" {
		return nil
	}
	return append([]string(nil), m.candidates[short]...)
}

func (m *semaLazyPlatformTypeMemberModel) hasField(key string) bool {
	if m == nil || key == "" {
		return false
	}
	m.fieldNamesOnce.Do(func() {
		fields := make(map[string]struct{})
		for i := range m.symbols {
			for _, member := range m.symbols[i].Members {
				if member.Kind == apexast.DeclarationField || member.Kind == apexast.DeclarationProperty {
					fields[normalizeName(member.Name)] = struct{}{}
				}
			}
		}
		m.fieldNames = fields
	})
	_, ok := m.fieldNames[key]
	return ok
}

func semaTypeMembersFromPlatformSymbol(symbol typesys.TypeSymbol) typeMembers {
	members := typeMembers{
		name:       semaTypeMembersName(symbol),
		shortKey:   semaShortTypeKey(symbol.Name),
		namespace:  symbol.Namespace,
		dependency: true,
		kind:       symbol.Kind,
		superClass: symbol.SuperClass,
		interfaces: append([]string(nil), symbol.Interfaces...),
		modifiers:  append([]string(nil), symbol.Modifiers...),
		methods:    make(map[string][]typesys.MemberSymbol),
		fields:     make(map[string]typesys.MemberSymbol),
	}
	for _, member := range symbol.Members {
		member = semaCloneMemberSymbol(member)
		switch member.Kind {
		case apexast.DeclarationMethod:
			key := normalizeName(member.Name)
			members.methods[key] = append(members.methods[key], member)
		case apexast.DeclarationConstructor:
			members.constructors = append(members.constructors, member)
		case apexast.DeclarationField, apexast.DeclarationProperty:
			members.fields[normalizeName(member.Name)] = member
		}
	}
	return members
}

const semaCurrentTypeScopeKey = "__glade_current_type"

const semaInferenceDepthScopeKey = "__glade_inference_depth"

const semaSyntheticStandardSObjectFieldModifier = "__glade_standard_sobject_field"

// semaMemberParameterNames returns the case-insensitive parameter name set for
// a method or constructor. Locals and nested bindings may not reuse these names.
func semaMemberParameterNames(member typesys.MemberSymbol) map[string]struct{} {
	if len(member.Parameters) == 0 {
		return nil
	}
	out := make(map[string]struct{}, len(member.Parameters))
	for _, param := range member.Parameters {
		name := strings.TrimSpace(param.Name)
		if name == "" {
			continue
		}
		out[normalizeName(name)] = struct{}{}
	}
	return out
}

func (a *Analyzer) checkMethodBodies(index typesys.Index) []diagnostic.Diagnostic {
	return a.checkMethodBodiesWithRecorder(index, nil)
}

func (a *Analyzer) checkMethodBodiesWithRecorder(index typesys.Index, recorder *perfRecorder) []diagnostic.Diagnostic {
	return a.checkMethodBodiesWithState(index, buildSemaTypeMemberState(index, recorder, a.sources), recorder)
}

func buildSemaTypeMemberState(index typesys.Index, recorder *perfRecorder, sourceResolvers ...*semaSources) *semaTypeMemberState {
	modelStarted := recorder.beginPhase()
	var sources *semaSources
	if len(sourceResolvers) > 0 {
		sources = sourceResolvers[0]
	}
	model := buildTypeMembersWithSources(index, sources)
	if recorder != nil {
		recorder.endPhase(&recorder.counters.TypeMemberModel, modelStarted)
	}
	return newSemaTypeMemberState(model)
}

func (a *Analyzer) checkMethodBodiesWithState(index typesys.Index, state *semaTypeMemberState, recorder *perfRecorder) []diagnostic.Diagnostic {
	return a.checkMethodBodiesWithView(index, state.view(), recorder)
}

func (a *Analyzer) checkMethodBodiesWithView(index typesys.Index, model *semaTypeMemberView, recorder *perfRecorder) []diagnostic.Diagnostic {
	return a.checkMethodBodiesWithViewWorkers(index, model, recorder, 1)
}

func (a *Analyzer) checkMethodBodiesWithViewWorkers(index typesys.Index, model *semaTypeMemberView, recorder *perfRecorder, workerCount int) []diagnostic.Diagnostic {
	constructability := buildConstructability(index)
	duplicateTypes := semaDuplicateTypeKeys(index)
	sources := a.sources
	if sources == nil {
		sources = newSemaSources(nil, recorder)
	}
	var diagnostics []diagnostic.Diagnostic
	bodyStarted := recorder.beginPhase()
	workItems := buildSemaMethodBodyWorkItems(index, sources)
	if workerCount > 1 {
		tasks := buildSemaMethodBodyTasks(index, workItems)
		if workerCount > len(tasks) {
			workerCount = len(tasks)
		}
		if workerCount > 1 {
			results := make([]semaMethodBodyResult, len(index.Types))
			views := make([]*semaTypeMemberView, workerCount)
			views[0] = model
			for workerID := 1; workerID < workerCount; workerID++ {
				views[workerID] = model.state.view()
			}
			var workers sync.WaitGroup
			for workerID := 0; workerID < workerCount; workerID++ {
				workerID := workerID
				workers.Add(1)
				go func() {
					defer workers.Done()
					for taskIndex := workerID; taskIndex < len(tasks); taskIndex += workerCount {
						task := tasks[taskIndex]
						results[task.typeIndex] = a.checkSemaMethodBodyTask(index, task, workItems, views[workerID], duplicateTypes, constructability)
					}
				}()
			}
			workers.Wait()
			for typeIndex := range index.Types {
				result := results[typeIndex]
				if result.recovered != nil {
					panic(result.recovered)
				}
				diagnostics = append(diagnostics, result.diagnostics...)
			}
			if recorder != nil {
				recorder.endPhase(&recorder.counters.MethodBodies, bodyStarted)
			}
			return diagnostics
		}
	}
	workItemIndex := 0
	for typeIndex, typ := range index.Types {
		if skipProjectDiagnosticType(typ) {
			continue
		}
		bodyModel := model
		if duplicateTypes[semaTypeSymbolKey(typ)] > 1 {
			bodyModel = semaModelWithCurrentType(model, typ)
		}
		for workItemIndex < len(workItems) && workItems[workItemIndex].typeIndex == typeIndex {
			item := workItems[workItemIndex]
			workItemIndex++
			itemType, member, ok := resolveSemaMethodBodyWorkItem(index, item)
			if !ok {
				continue
			}
			diagnostics = append(diagnostics, a.checkBodyText(itemType, member, item.body, item.bodyOffset, item.source, bodyModel, constructability)...)
		}
	}
	if recorder != nil {
		recorder.endPhase(&recorder.counters.MethodBodies, bodyStarted)
	}
	return diagnostics
}

// semaTriggerContextTypeKey is the member-model key for the platform `Trigger`
// context type.
const semaTriggerContextTypeKey = "trigger"

type semaTriggerBodyWorkItem struct {
	triggerIndex int
	body         string
	bodyOffset   int
	source       string
}

func buildSemaTriggerBodyWorkItems(index typesys.Index, sources *semaSources) []semaTriggerBodyWorkItem {
	var workItems []semaTriggerBodyWorkItem
	for triggerIndex, trigger := range index.Triggers {
		if trigger.Dependency || trigger.File == "" {
			continue
		}
		source, ok := sources.normalizedForTrigger(trigger)
		if !ok {
			continue
		}
		body, bodyOffset, extracted := extractBodyForSema(source, trigger.Range)
		if !extracted {
			continue
		}
		workItems = append(workItems, semaTriggerBodyWorkItem{
			triggerIndex: triggerIndex,
			body:         body,
			bodyOffset:   bodyOffset,
			source:       source,
		})
	}
	return workItems
}

func (a *Analyzer) checkTriggerBodiesWithView(index typesys.Index, model *semaTypeMemberView, recorder *perfRecorder) []diagnostic.Diagnostic {
	sources := a.sources
	if sources == nil {
		sources = newSemaSources(nil, recorder)
	}
	bodyStarted := recorder.beginPhase()
	workItems := buildSemaTriggerBodyWorkItems(index, sources)
	var diagnostics []diagnostic.Diagnostic
	if len(workItems) > 0 {
		constructability := buildConstructability(index)
		for _, item := range workItems {
			trigger := index.Triggers[item.triggerIndex]
			typ, member := semaTriggerBodyDeclaration(trigger)
			bodyModel := semaModelWithTriggerScope(model, trigger)
			diagnostics = append(diagnostics, a.checkBodyText(typ, member, item.body, item.bodyOffset, item.source, bodyModel, constructability)...)
		}
	}
	if recorder != nil {
		recorder.endPhase(&recorder.counters.MethodBodies, bodyStarted)
	}
	return diagnostics
}

// semaTriggerBodyDeclaration models a trigger body as the static void member it
// behaves like: no owning class state, no parameters, and no return value.
func semaTriggerBodyDeclaration(trigger typesys.TriggerSymbol) (typesys.TypeSymbol, typesys.MemberSymbol) {
	typ := typesys.TypeSymbol{
		Kind:      apexast.DeclarationClass,
		Name:      trigger.Name,
		File:      trigger.File,
		Namespace: trigger.Namespace,
		Range:     trigger.Range,
	}
	member := typesys.MemberSymbol{
		Kind:      apexast.DeclarationTrigger,
		Name:      trigger.Name,
		Type:      "void",
		Modifiers: []string{"static"},
		Range:     trigger.Range,
	}
	return typ, member
}

// semaModelWithTriggerScope specializes the platform `Trigger` context to the
// object the trigger is declared on, matching how Salesforce types
// Trigger.new/old/newMap/oldMap inside a trigger body.
func semaModelWithTriggerScope(model *semaTypeMemberView, trigger typesys.TriggerSymbol) *semaTypeMemberView {
	objectName := strings.TrimSpace(trigger.ObjectName)
	if objectName == "" {
		return model
	}
	context, ok := model.lookup(semaTriggerContextTypeKey)
	if !ok {
		return model
	}
	context = semaCloneTypeMembers(context)
	recordType := map[string]string{
		"new":    "List<" + objectName + ">",
		"old":    "List<" + objectName + ">",
		"newmap": "Map<Id," + objectName + ">",
		"oldmap": "Map<Id," + objectName + ">",
	}
	for key, fieldType := range recordType {
		field, exists := context.fields[key]
		if !exists {
			continue
		}
		field.Type = fieldType
		context.fields[key] = field
	}
	out := &semaTypeMemberView{
		state:    model.state,
		current:  make(map[string]typeMembers, len(model.current)+1),
		hydrated: model.hydrated,
	}
	for key, members := range model.current {
		out.current[key] = members
	}
	out.current[semaTriggerContextTypeKey] = context
	return out
}

type semaMethodBodyTask struct {
	typeIndex     int
	workItemStart int
	workItemEnd   int
}

type semaMethodBodyResult struct {
	diagnostics []diagnostic.Diagnostic
	recovered   any
}

func buildSemaMethodBodyTasks(index typesys.Index, workItems []semaMethodBodyWorkItem) []semaMethodBodyTask {
	tasks := make([]semaMethodBodyTask, 0, len(index.Types))
	workItemIndex := 0
	for typeIndex, typ := range index.Types {
		if skipProjectDiagnosticType(typ) {
			continue
		}
		workItemStart := workItemIndex
		for workItemIndex < len(workItems) && workItems[workItemIndex].typeIndex == typeIndex {
			workItemIndex++
		}
		tasks = append(tasks, semaMethodBodyTask{
			typeIndex:     typeIndex,
			workItemStart: workItemStart,
			workItemEnd:   workItemIndex,
		})
	}
	return tasks
}

func (a *Analyzer) checkSemaMethodBodyTask(index typesys.Index, task semaMethodBodyTask, workItems []semaMethodBodyWorkItem, model *semaTypeMemberView, duplicateTypes map[string]int, constructability map[string]typesys.TypeSymbol) (result semaMethodBodyResult) {
	defer func() {
		result.recovered = recover()
	}()
	typ := index.Types[task.typeIndex]
	bodyModel := model
	if duplicateTypes[semaTypeSymbolKey(typ)] > 1 {
		bodyModel = semaModelWithCurrentType(model, typ)
	}
	for workItemIndex := task.workItemStart; workItemIndex < task.workItemEnd; workItemIndex++ {
		item := workItems[workItemIndex]
		itemType, member, ok := resolveSemaMethodBodyWorkItem(index, item)
		if !ok {
			continue
		}
		result.diagnostics = append(result.diagnostics, a.checkBodyText(itemType, member, item.body, item.bodyOffset, item.source, bodyModel, constructability)...)
	}
	return result
}

type semaMethodBodyWorkItem struct {
	typeIndex     int
	memberIndex   int
	accessorIndex int
	body          string
	bodyOffset    int
	source        string
}

const semaMethodBodyNoAccessor = -1

type semaMethodBodySource struct {
	typeIndex int
	source    string
}

func buildSemaMethodBodyWorkItems(index typesys.Index, sources *semaSources) []semaMethodBodyWorkItem {
	bodySources, workItemCount := collectSemaMethodBodySources(index, sources)
	workItems := make([]semaMethodBodyWorkItem, 0, workItemCount)
	for _, bodySource := range bodySources {
		typeIndex := bodySource.typeIndex
		typ := index.Types[typeIndex]
		source := bodySource.source
		appendBody := func(memberIndex, accessorIndex int, bodyRange diagnostic.Range) {
			body, bodyOffset, extracted := extractBodyForSema(source, bodyRange)
			if !extracted {
				return
			}
			workItems = append(workItems, semaMethodBodyWorkItem{
				typeIndex:     typeIndex,
				memberIndex:   memberIndex,
				accessorIndex: accessorIndex,
				body:          body,
				bodyOffset:    bodyOffset,
				source:        source,
			})
		}
		for memberIndex, member := range typ.Members {
			switch member.Kind {
			case apexast.DeclarationMethod, apexast.DeclarationConstructor, apexast.DeclarationInitializer:
				appendBody(memberIndex, semaMethodBodyNoAccessor, member.Range)
			case apexast.DeclarationProperty:
				for accessorIndex, accessor := range member.Accessors {
					if accessor.HasBody {
						appendBody(memberIndex, accessorIndex, accessor.Range)
					}
				}
			}
		}
	}
	return workItems
}

func resolveSemaMethodBodyWorkItem(index typesys.Index, item semaMethodBodyWorkItem) (typesys.TypeSymbol, typesys.MemberSymbol, bool) {
	if item.typeIndex < 0 || item.typeIndex >= len(index.Types) {
		return typesys.TypeSymbol{}, typesys.MemberSymbol{}, false
	}
	typ := index.Types[item.typeIndex]
	if item.memberIndex < 0 || item.memberIndex >= len(typ.Members) {
		return typesys.TypeSymbol{}, typesys.MemberSymbol{}, false
	}
	member := typ.Members[item.memberIndex]
	if item.accessorIndex == semaMethodBodyNoAccessor {
		switch member.Kind {
		case apexast.DeclarationMethod, apexast.DeclarationConstructor, apexast.DeclarationInitializer:
			return typ, member, true
		default:
			return typesys.TypeSymbol{}, typesys.MemberSymbol{}, false
		}
	}
	if member.Kind != apexast.DeclarationProperty || item.accessorIndex < 0 || item.accessorIndex >= len(member.Accessors) {
		return typesys.TypeSymbol{}, typesys.MemberSymbol{}, false
	}
	accessor := member.Accessors[item.accessorIndex]
	if !accessor.HasBody {
		return typesys.TypeSymbol{}, typesys.MemberSymbol{}, false
	}
	member.Kind = apexast.DeclarationMethod
	member.Name += "." + accessor.Kind
	if accessor.Kind == "set" {
		propertyType := member.Type
		member.Type = "void"
		member.Parameters = []apexast.Parameter{{Name: "value", Type: propertyType}}
	}
	return typ, member, true
}

func collectSemaMethodBodySources(index typesys.Index, sources *semaSources) ([]semaMethodBodySource, int) {
	var bodySources []semaMethodBodySource
	workItemCount := 0
	for typeIndex, typ := range index.Types {
		if skipProjectDiagnosticType(typ) || !semaTypeHasMethodBodyWork(typ) {
			continue
		}
		source, ok := sources.normalizedForType(typ)
		if !ok {
			continue
		}
		count := countExtractableSemaMethodBodyRanges(typ, source)
		if count == 0 {
			continue
		}
		bodySources = append(bodySources, semaMethodBodySource{typeIndex: typeIndex, source: source})
		workItemCount += count
	}
	return bodySources, workItemCount
}

func countExtractableSemaMethodBodyRanges(typ typesys.TypeSymbol, source string) int {
	count := 0
	countRange := func(bodyRange diagnostic.Range) {
		if _, _, extracted := extractBodyForSema(source, bodyRange); extracted {
			count++
		}
	}
	for _, member := range typ.Members {
		switch member.Kind {
		case apexast.DeclarationMethod, apexast.DeclarationConstructor, apexast.DeclarationInitializer:
			countRange(member.Range)
		case apexast.DeclarationProperty:
			for _, accessor := range member.Accessors {
				if accessor.HasBody {
					countRange(accessor.Range)
				}
			}
		}
	}
	return count
}

func semaTypeHasMethodBodyWork(typ typesys.TypeSymbol) bool {
	for _, member := range typ.Members {
		switch member.Kind {
		case apexast.DeclarationMethod, apexast.DeclarationConstructor, apexast.DeclarationInitializer:
			return true
		case apexast.DeclarationProperty:
			for _, accessor := range member.Accessors {
				if accessor.HasBody {
					return true
				}
			}
		}
	}
	return false
}

func semaDuplicateTypeKeys(index typesys.Index) map[string]int {
	counts := make(map[string]int)
	for _, typ := range index.Types {
		counts[semaTypeSymbolKey(typ)]++
	}
	return counts
}

func semaTypeSymbolKey(typ typesys.TypeSymbol) string {
	if typ.Namespace != "" {
		return normalizeName(typ.Namespace + "." + typ.Name)
	}
	return normalizeName(typ.Name)
}

func semaModelWithCurrentType(model *semaTypeMemberView, typ typesys.TypeSymbol) *semaTypeMemberView {
	out := &semaTypeMemberView{
		state:    model.state,
		current:  make(map[string]typeMembers, 2),
		hydrated: model.hydrated,
	}
	members := semaTypeMembersFromSymbol(typ)
	out.current[normalizeName(typ.Name)] = members
	if typ.Namespace != "" {
		out.current[normalizeName(typ.Namespace+"."+typ.Name)] = members
	}
	members.superClass = resolveNestedTypeName(out, members.name, members.superClass)
	for i, iface := range members.interfaces {
		members.interfaces[i] = resolveNestedTypeName(out, members.name, iface)
	}
	for fieldKey, field := range members.fields {
		field.Type = resolveNestedTypeReference(out, members.name, field.Type)
		members.fields[fieldKey] = field
	}
	for methodKey, overloads := range members.methods {
		for i := range overloads {
			overloads[i].Type = resolveNestedTypeReference(out, members.name, overloads[i].Type)
			for j := range overloads[i].Parameters {
				overloads[i].Parameters[j].Type = resolveNestedTypeReference(out, members.name, overloads[i].Parameters[j].Type)
			}
		}
		members.methods[methodKey] = overloads
	}
	for i := range members.constructors {
		for j := range members.constructors[i].Parameters {
			members.constructors[i].Parameters[j].Type = resolveNestedTypeReference(out, members.name, members.constructors[i].Parameters[j].Type)
		}
	}
	out.current[normalizeName(typ.Name)] = members
	if typ.Namespace != "" {
		out.current[normalizeName(typ.Namespace+"."+typ.Name)] = members
	}
	return out
}

func semaTypeMembersFromSymbol(typ typesys.TypeSymbol) typeMembers {
	members := typeMembers{
		name:         semaTypeMembersName(typ),
		shortKey:     semaShortTypeKey(typ.Name),
		namespace:    typ.Namespace,
		dependency:   typ.Dependency,
		nestingDepth: typ.NestingDepth,
		kind:         typ.Kind,
		superClass:   typ.SuperClass,
		interfaces:   append([]string(nil), typ.Interfaces...),
		modifiers:    append([]string(nil), typ.Modifiers...),
		methods:      make(map[string][]typesys.MemberSymbol),
		fields:       make(map[string]typesys.MemberSymbol),
	}
	for _, member := range typ.Members {
		member = semaCloneMemberSymbol(member)
		switch member.Kind {
		case apexast.DeclarationMethod:
			members.methods[normalizeName(member.Name)] = append(members.methods[normalizeName(member.Name)], member)
		case apexast.DeclarationConstructor:
			members.constructors = append(members.constructors, member)
		case apexast.DeclarationField, apexast.DeclarationProperty:
			members.fields[normalizeName(member.Name)] = member
		}
	}
	if typ.Kind == apexast.DeclarationEnum {
		for _, value := range semaEnumValues(typ) {
			members.fields[normalizeName(value)] = typesys.MemberSymbol{
				Kind:      apexast.DeclarationField,
				Name:      value,
				Type:      typ.Name,
				Modifiers: []string{"public", "static"},
			}
		}
	}
	return members
}

func buildTypeMembers(index typesys.Index) *semaTypeMemberModel {
	return buildTypeMembersWithSources(index, newSemaSources(nil, nil))
}

func buildTypeMembersWithSources(index typesys.Index, sources *semaSources) *semaTypeMemberModel {
	return buildTypeMemberLayerWithSources(index, sources, semaPlatformTypeMemberModel())
}

func buildTypeMemberLayerWithSources(index typesys.Index, sources *semaSources, platform *semaTypeMemberModel) *semaTypeMemberModel {
	if sources == nil {
		sources = newSemaSources(nil, nil)
	}
	out := make(map[string]typeMembers)
	shortAliases := make(map[string][]string)
	projectNamespace := index.Project.Namespace
	for _, typ := range index.Types {
		members := typeMembers{
			name:         semaTypeMembersName(typ),
			shortKey:     semaShortTypeKey(typ.Name),
			namespace:    typ.Namespace,
			dependency:   typ.Dependency,
			nestingDepth: typ.NestingDepth,
			kind:         typ.Kind,
			superClass:   typ.SuperClass,
			interfaces:   append([]string(nil), typ.Interfaces...),
			modifiers:    append([]string(nil), typ.Modifiers...),
			methods:      make(map[string][]typesys.MemberSymbol),
			fields:       make(map[string]typesys.MemberSymbol),
		}
		for _, member := range typ.Members {
			member = semaCloneMemberSymbol(member)
			switch member.Kind {
			case apexast.DeclarationMethod:
				members.methods[normalizeName(member.Name)] = append(members.methods[normalizeName(member.Name)], member)
			case apexast.DeclarationConstructor:
				members.constructors = append(members.constructors, member)
			case apexast.DeclarationField, apexast.DeclarationProperty:
				members.fields[normalizeName(member.Name)] = member
			case apexast.DeclarationEnum:
				nestedName := typ.Name + "." + member.Name
				enumMembers := typeMembers{
					name:       semaQualifiedDependencyTypeName(typ.Namespace, nestedName, typ.Dependency, typ.Artifact, typ.SourceRoot),
					shortKey:   semaShortTypeKey(nestedName),
					namespace:  typ.Namespace,
					dependency: typ.Dependency,
					kind:       apexast.DeclarationEnum,
					methods:    make(map[string][]typesys.MemberSymbol),
					fields:     make(map[string]typesys.MemberSymbol),
				}
				enumType := typesys.TypeSymbol{
					Kind:  apexast.DeclarationEnum,
					Name:  nestedName,
					File:  typ.File,
					Range: member.Range,
				}
				enumType.Namespace = typ.Namespace
				enumType.SourceNamespaceRemaps = append(enumType.SourceNamespaceRemaps, typ.SourceNamespaceRemaps...)
				enumType.SourceRoot = typ.SourceRoot
				enumType.Version = typ.Version
				enumType.Dependency = typ.Dependency
				for _, value := range semaEnumValuesWithSources(enumType, sources) {
					enumMembers.fields[normalizeName(value)] = typesys.MemberSymbol{
						Kind:      apexast.DeclarationField,
						Name:      value,
						Type:      nestedName,
						Modifiers: []string{"public", "static"},
					}
				}
				if !semaRequiresQualifiedDependencyName(typ) {
					key := normalizeName(nestedName)
					if semaShouldStoreTypeMembers(out[key], typ) {
						out[key] = enumMembers
					}
					shortAliases[normalizeName(member.Name)] = append(shortAliases[normalizeName(member.Name)], nestedName)
				}
				if typ.Namespace != "" {
					out[normalizeName(typ.Namespace+"."+nestedName)] = enumMembers
				}
			}
		}
		if typ.Kind == apexast.DeclarationEnum {
			for _, value := range semaEnumValuesWithSources(typ, sources) {
				members.fields[normalizeName(value)] = typesys.MemberSymbol{
					Kind:      apexast.DeclarationField,
					Name:      value,
					Type:      typ.Name,
					Modifiers: []string{"public", "static"},
				}
			}
		}
		if !semaRequiresQualifiedDependencyName(typ) {
			key := normalizeName(typ.Name)
			if semaShouldStoreTypeMembers(out[key], typ) {
				out[key] = members
			}
		}
		if typ.Namespace != "" {
			key := normalizeName(typ.Namespace + "." + typ.Name)
			if semaShouldStoreTypeMembers(out[key], typ) {
				out[key] = members
			}
		}
		if short := shortNestedTypeName(typ.Name); !semaGeneratedFlowInterviewType(typ) && !semaRequiresQualifiedDependencyName(typ) && short != typ.Name {
			shortAliases[normalizeName(short)] = append(shortAliases[normalizeName(short)], typ.Name)
		}
	}
	for _, object := range index.Objects {
		objectKey := normalizeName(object.Name)
		objectMembers, objectOK := out[objectKey]
		if objectOK && !objectMembers.sobject && !semaShouldMergeStandardSObjectMembers(object.Name, objectMembers) {
			continue
		}
		if !objectOK {
			objectMembers = typeMembers{
				name:                   object.Name,
				shortKey:               semaShortTypeKey(object.Name),
				namespace:              semaNamespaceFromAPIName(object.Name),
				sobject:                true,
				externalPackageSObject: semaIsExternalManagedPackageAPIName(projectNamespace, object.Name),
				partialSObject:         object.Partial,
				kind:                   apexast.DeclarationClass,
				fields:                 make(map[string]typesys.MemberSymbol),
				methods:                make(map[string][]typesys.MemberSymbol),
			}
		}
		objectMembers.sobject = true
		objectMembers.partialSObject = objectMembers.partialSObject || object.Partial
		if objectMembers.namespace == "" {
			objectMembers.namespace = semaNamespaceFromAPIName(object.Name)
		}
		objectMembers.externalPackageSObject = semaIsExternalManagedPackageAPIName(projectNamespace, object.Name)
		if objectMembers.fields == nil {
			objectMembers.fields = make(map[string]typesys.MemberSymbol)
		}
		semaAddSObjectProviderMembers(&objectMembers, newSemaSObjectFieldProvider(projectNamespace, object), projectNamespace, index.Objects)
		for _, field := range object.Fields {
			childRelationshipNames := []string{}
			if field.ChildRelationshipName != "" {
				childRelationshipNames = append(childRelationshipNames, field.ChildRelationshipName)
			}
			if len(childRelationshipNames) == 0 {
				continue
			}
			for _, parent := range field.ReferenceTo {
				parentKey := normalizeName(parent)
				if parentKey == "" {
					continue
				}
				parentMembers := objectMembers
				if parentKey != objectKey {
					var ok bool
					parentMembers, ok = out[parentKey]
					if !ok {
						parentMembers = typeMembers{
							name:     parent,
							shortKey: semaShortTypeKey(parent),
							sobject:  true,
							kind:     apexast.DeclarationClass,
							fields:   make(map[string]typesys.MemberSymbol),
							methods:  make(map[string][]typesys.MemberSymbol),
						}
					}
				} else if parentMembers.fields == nil {
					parentMembers = typeMembers{
						name:     object.Name,
						shortKey: semaShortTypeKey(object.Name),
						sobject:  true,
						kind:     apexast.DeclarationClass,
						fields:   make(map[string]typesys.MemberSymbol),
						methods:  make(map[string][]typesys.MemberSymbol),
					}
				}
				parentMembers.sobject = true
				if parentMembers.fields == nil {
					parentMembers.fields = make(map[string]typesys.MemberSymbol)
				}
				for _, childRelationshipName := range childRelationshipNames {
					childRelationship := typesys.MemberSymbol{
						Kind:      apexast.DeclarationField,
						Name:      childRelationshipName,
						Type:      "List<" + object.Name + ">",
						Modifiers: []string{"public"},
					}
					if field.ChildRelationshipNameInferred {
						semaAddSyntheticChildRelationshipAlias(&parentMembers, projectNamespace, childRelationship)
					} else {
						semaAddDeclaredChildRelationshipMember(&parentMembers, projectNamespace, childRelationship)
					}
					if strings.HasSuffix(childRelationshipName, "__r") {
						continue
					}
					childRelationshipAlias := childRelationshipName + "__r"
					semaAddSyntheticChildRelationshipAlias(&parentMembers, projectNamespace, typesys.MemberSymbol{
						Kind:      apexast.DeclarationField,
						Name:      childRelationshipAlias,
						Type:      "List<" + object.Name + ">",
						Modifiers: []string{"public"},
					})
				}
				if parentKey == objectKey {
					objectMembers = parentMembers
				} else {
					semaStoreSObjectTypeMembers(out, projectNamespace, parent, parentMembers)
				}
			}
		}
		semaStoreSObjectTypeMembers(out, projectNamespace, object.Name, objectMembers)
	}
	semaApplyPlatformInterfaceOverlays(out)
	semaApplyPlatformFieldOverlays(out)
	for short, names := range shortAliases {
		if len(names) == 1 {
			if _, exists := out[short]; exists {
				continue
			}
			out[short] = out[normalizeName(names[0])]
		}
	}
	model := &semaTypeMemberModel{members: out}
	view := newSemaTypeMemberStateWithPlatform(model, platform).view()
	for key, members := range out {
		if members.shortKey == "" {
			members.shortKey = semaShortTypeKey(members.name)
		}
		if members.syntheticStandardSObject {
			out[key] = members
			continue
		}
		members.superClass = resolveNestedTypeName(view, members.name, members.superClass)
		for i, iface := range members.interfaces {
			members.interfaces[i] = resolveNestedTypeName(view, members.name, iface)
		}
		for fieldKey, field := range members.fields {
			field.Type = resolveNestedTypeReference(view, members.name, field.Type)
			members.fields[fieldKey] = field
		}
		for methodKey, overloads := range members.methods {
			for i := range overloads {
				overloads[i].Type = resolveNestedTypeReference(view, members.name, overloads[i].Type)
				for j := range overloads[i].Parameters {
					overloads[i].Parameters[j].Type = resolveNestedTypeReference(view, members.name, overloads[i].Parameters[j].Type)
				}
			}
			members.methods[methodKey] = overloads
		}
		for i := range members.constructors {
			for j := range members.constructors[i].Parameters {
				members.constructors[i].Parameters[j].Type = resolveNestedTypeReference(view, members.name, members.constructors[i].Parameters[j].Type)
			}
		}
		out[key] = members
	}
	model.shortCandidates = buildSemaShortCandidateIndex(out, semaTypeMemberCandidateKeys(index))
	return model
}

func buildSemaTypeMemberView(index typesys.Index) *semaTypeMemberView {
	return newSemaTypeMemberState(buildTypeMembers(index)).view()
}

func semaRequiresQualifiedDependencyName(typ typesys.TypeSymbol) bool {
	return semaRequiresQualifiedDependencyNameValues(typ.Namespace, typ.Dependency, typ.Artifact, typ.SourceRoot)
}

func semaGeneratedFlowInterviewType(typ typesys.TypeSymbol) bool {
	return strings.EqualFold(typ.SuperClass, "Flow.Interview") && strings.HasPrefix(strings.ToLower(typ.Name), "flow.interview.")
}

func semaCloneMemberSymbol(member typesys.MemberSymbol) typesys.MemberSymbol {
	member.Modifiers = append([]string(nil), member.Modifiers...)
	member.Annotations = append([]apexast.Annotation(nil), member.Annotations...)
	member.Parameters = semaCloneParameters(member.Parameters)
	member.Accessors = semaCloneAccessors(member.Accessors)
	return member
}

func semaCloneTypeMembers(members typeMembers) typeMembers {
	clone := members
	if members.interfaces != nil {
		clone.interfaces = append([]string(nil), members.interfaces...)
	}
	if members.modifiers != nil {
		clone.modifiers = append([]string(nil), members.modifiers...)
	}
	if members.methods != nil {
		clone.methods = make(map[string][]typesys.MemberSymbol, len(members.methods))
		for key, overloads := range members.methods {
			copied := make([]typesys.MemberSymbol, len(overloads))
			for i, overload := range overloads {
				copied[i] = semaCloneMemberSymbol(overload)
			}
			clone.methods[key] = copied
		}
	}
	if members.constructors != nil {
		clone.constructors = make([]typesys.MemberSymbol, len(members.constructors))
		for i, constructor := range members.constructors {
			clone.constructors[i] = semaCloneMemberSymbol(constructor)
		}
	}
	if members.fields != nil {
		clone.fields = make(map[string]typesys.MemberSymbol, len(members.fields))
		for key, field := range members.fields {
			clone.fields[key] = semaCloneMemberSymbol(field)
		}
	}
	if members.syntheticChildRelationshipAliases != nil {
		clone.syntheticChildRelationshipAliases = make(map[string]struct{}, len(members.syntheticChildRelationshipAliases))
		for key := range members.syntheticChildRelationshipAliases {
			clone.syntheticChildRelationshipAliases[key] = struct{}{}
		}
	}
	return clone
}

func semaCloneParameters(parameters []apexast.Parameter) []apexast.Parameter {
	if len(parameters) == 0 {
		return nil
	}
	out := make([]apexast.Parameter, len(parameters))
	for i, parameter := range parameters {
		out[i] = parameter
		out[i].Modifiers = append([]string(nil), parameter.Modifiers...)
	}
	return out
}

func semaCloneAccessors(accessors []apexast.Accessor) []apexast.Accessor {
	if len(accessors) == 0 {
		return nil
	}
	out := make([]apexast.Accessor, len(accessors))
	for i, accessor := range accessors {
		out[i] = accessor
		out[i].Modifiers = append([]string(nil), accessor.Modifiers...)
	}
	return out
}

func semaTypeMembersName(typ typesys.TypeSymbol) string {
	return semaQualifiedDependencyTypeName(typ.Namespace, typ.Name, typ.Dependency, typ.Artifact, typ.SourceRoot)
}

func semaQualifiedDependencyTypeName(namespace, name string, dependency, artifact bool, sourceRoot string) string {
	if semaRequiresQualifiedDependencyNameValues(namespace, dependency, artifact, sourceRoot) {
		return namespace + "." + name
	}
	return name
}

func semaRequiresQualifiedDependencyNameValues(namespace string, dependency, artifact bool, sourceRoot string) bool {
	return dependency && namespace != "" && (artifact || sourceRoot != "")
}

func semaShouldStoreTypeMembers(existing typeMembers, typ typesys.TypeSymbol) bool {
	if existing.name == "" {
		return true
	}
	return existing.dependency && !typ.Dependency
}

func semaTypeMemberCandidateKeys(index typesys.Index) []string {
	keys := make([]string, 0, len(index.Types)*2)
	for _, typ := range index.Types {
		for _, member := range typ.Members {
			if member.Kind != apexast.DeclarationEnum {
				continue
			}
			nestedName := typ.Name + "." + member.Name
			if !semaRequiresQualifiedDependencyName(typ) {
				keys = append(keys, normalizeName(nestedName))
			}
			if typ.Namespace != "" {
				keys = append(keys, normalizeName(typ.Namespace+"."+nestedName))
			}
		}
		if !semaRequiresQualifiedDependencyName(typ) {
			keys = append(keys, normalizeName(typ.Name))
		}
		if typ.Namespace != "" {
			keys = append(keys, normalizeName(typ.Namespace+"."+typ.Name))
		}
		if short := shortNestedTypeName(typ.Name); !semaGeneratedFlowInterviewType(typ) && !semaRequiresQualifiedDependencyName(typ) && short != typ.Name {
			keys = append(keys, normalizeName(short))
		}
	}
	return keys
}

func buildSemaShortCandidateIndex(model map[string]typeMembers, preferredKeys []string) map[string][]string {
	index := make(map[string][]string)
	seen := make(map[string]bool, len(model))
	add := func(key string) {
		if seen[key] {
			return
		}
		members, ok := model[key]
		if !ok {
			return
		}
		seen[key] = true
		short := members.shortKey
		if short == "" {
			short = semaShortTypeKeyFromNormalizedKey(key)
		}
		if short == "" {
			return
		}
		index[short] = append(index[short], key)
	}
	for _, key := range preferredKeys {
		add(key)
	}
	remaining := make([]string, 0, len(model)-len(seen))
	for key := range model {
		if !seen[key] {
			remaining = append(remaining, key)
		}
	}
	sort.Strings(remaining)
	for _, key := range remaining {
		add(key)
	}
	return index
}

func semaTypeMemberViewFromMembers(members map[string]typeMembers) *semaTypeMemberView {
	model := &semaTypeMemberModel{
		members:         members,
		shortCandidates: buildSemaShortCandidateIndex(members, nil),
	}
	return newSemaTypeMemberState(model).view()
}

func semaAddSchemaFieldMember(fields map[string]typesys.MemberSymbol, namespace string, member typesys.MemberSymbol) {
	if fields == nil || strings.TrimSpace(member.Name) == "" {
		return
	}
	fields[normalizeName(member.Name)] = member
	if localName, ok := semaProjectLocalAPIName(namespace, member.Name); ok {
		alias := member
		alias.Name = localName
		fields[normalizeName(localName)] = alias
	}
}

func semaAddSchemaFieldMemberIfAbsent(fields map[string]typesys.MemberSymbol, namespace string, member typesys.MemberSymbol) {
	if fields == nil || strings.TrimSpace(member.Name) == "" {
		return
	}
	if _, exists := fields[normalizeName(member.Name)]; exists {
		return
	}
	fields[normalizeName(member.Name)] = member
	if localName, ok := semaProjectLocalAPIName(namespace, member.Name); ok {
		localKey := normalizeName(localName)
		if _, exists := fields[localKey]; !exists {
			alias := member
			alias.Name = localName
			fields[localKey] = alias
		}
	}
}

func semaAddSchemaParentRelationshipMember(members *typeMembers, namespace string, member typesys.MemberSymbol) {
	if members == nil || members.fields == nil || strings.TrimSpace(member.Name) == "" {
		return
	}
	key := normalizeName(member.Name)
	if _, exists := members.fields[key]; exists {
		if _, synthetic := members.syntheticChildRelationshipAliases[key]; !synthetic {
			return
		}
	}
	semaAddSchemaFieldMember(members.fields, namespace, member)
	delete(members.syntheticChildRelationshipAliases, key)
	if localName, ok := semaProjectLocalAPIName(namespace, member.Name); ok {
		delete(members.syntheticChildRelationshipAliases, normalizeName(localName))
	}
}

func semaAddDeclaredChildRelationshipMember(members *typeMembers, namespace string, member typesys.MemberSymbol) {
	if members == nil || members.fields == nil || strings.TrimSpace(member.Name) == "" {
		return
	}
	semaAddSchemaFieldMember(members.fields, namespace, member)
	delete(members.syntheticChildRelationshipAliases, normalizeName(member.Name))
	if localName, ok := semaProjectLocalAPIName(namespace, member.Name); ok {
		delete(members.syntheticChildRelationshipAliases, normalizeName(localName))
	}
}

func semaAddSyntheticChildRelationshipAlias(members *typeMembers, namespace string, member typesys.MemberSymbol) {
	if members == nil || members.fields == nil || strings.TrimSpace(member.Name) == "" {
		return
	}
	key := normalizeName(member.Name)
	if _, exists := members.fields[key]; exists {
		return
	}
	semaAddSchemaFieldMember(members.fields, namespace, member)
	if members.syntheticChildRelationshipAliases == nil {
		members.syntheticChildRelationshipAliases = make(map[string]struct{})
	}
	members.syntheticChildRelationshipAliases[key] = struct{}{}
	if localName, ok := semaProjectLocalAPIName(namespace, member.Name); ok {
		members.syntheticChildRelationshipAliases[normalizeName(localName)] = struct{}{}
	}
}

func semaLocationComponentFieldNames(fieldName string) []string {
	if !strings.HasSuffix(fieldName, "__c") {
		return nil
	}
	base := strings.TrimSuffix(fieldName, "__c")
	return []string{base + "__Latitude__s", base + "__Longitude__s"}
}

func semaStoreSObjectTypeMembers(out map[string]typeMembers, namespace, objectName string, members typeMembers) {
	key := normalizeName(objectName)
	if key == "" {
		return
	}
	out[key] = members
	if localName, ok := semaProjectLocalAPIName(namespace, objectName); ok {
		localKey := normalizeName(localName)
		if existing, exists := out[localKey]; !exists || existing.sobject {
			out[localKey] = members
		}
	}
}

func semaShortCandidateKeys(model *semaTypeMemberView, short string) []string {
	return model.shortCandidateKeys(short)
}

func semaShouldMergeStandardSObjectMembers(objectName string, members typeMembers) bool {
	if !members.sobject && !members.dependency {
		return false
	}
	if storage.IsKnownStandardObject(objectName) {
		return true
	}
	if !members.dependency {
		return false
	}
	switch normalizeName(objectName) {
	case "profile", "user", "userlicense":
		return true
	default:
		return false
	}
}

var semaStandardSObjectMembersCache struct {
	once      sync.Once
	names     []string
	nameByKey map[string]string
}

func ensureSemaStandardSObjectMembers() {
	semaStandardSObjectMembersCache.once.Do(func() {
		sourceNames := append(storage.KnownStandardObjectNames(), semaAdditionalStandardSObjectNames()...)
		sourceNames = append(sourceNames, semaStandardChangeEventNames(sourceNames)...)
		names := make([]string, 0, len(sourceNames))
		nameByKey := make(map[string]string, len(sourceNames))
		seen := make(map[string]bool, len(sourceNames))
		for _, objectName := range sourceNames {
			key := normalizeName(objectName)
			if key == "" || seen[key] {
				continue
			}
			seen[key] = true
			names = append(names, objectName)
			nameByKey[key] = objectName
		}
		semaStandardSObjectMembersCache.names = names
		semaStandardSObjectMembersCache.nameByKey = nameByKey
	})
}

func semaStandardSObjectPlaceholder(objectName string) typeMembers {
	return typeMembers{
		name:                     objectName,
		shortKey:                 semaShortTypeKey(objectName),
		sobject:                  true,
		kind:                     apexast.DeclarationClass,
		syntheticStandardSObject: true,
	}
}

func semaBuildStandardSObjectMembers(objectName string) typeMembers {
	members := semaStandardSObjectPlaceholder(objectName)
	members.fields = make(map[string]typesys.MemberSymbol)
	members.methods = make(map[string][]typesys.MemberSymbol)
	addStandardSObjectFields(&members, objectName, true)
	return members
}

func semaEnsureStandardSObjectTypeMembers(model *semaTypeMemberView, key string, members typeMembers) typeMembers {
	if !members.syntheticStandardSObject || members.standardSObjectFieldsLoaded {
		return members
	}
	hydrated := semaBuildStandardSObjectMembers(members.name)
	for fieldKey, field := range members.fields {
		hydrated.fields[fieldKey] = field
	}
	for methodKey, methods := range members.methods {
		hydrated.methods[methodKey] = append(hydrated.methods[methodKey], methods...)
	}
	if key == "" {
		key = normalizeName(members.name)
	}
	if key != "" {
		model.storeHydrated(key, hydrated)
	}
	return hydrated
}

func addStandardSObjectFields(members *typeMembers, objectName string, synthetic bool) {
	semaAddCommonSObjectFields(members.fields)
	members.fields[normalizeName("SObjectType")] = typesys.MemberSymbol{
		Kind:      apexast.DeclarationField,
		Name:      "SObjectType",
		Type:      "Schema.SObjectType",
		Modifiers: []string{"public", "static", semaSyntheticStandardSObjectFieldModifier},
	}
	definitionName := objectName
	isChangeEvent := false
	if baseName, ok := semaChangeEventBaseObjectName(objectName); ok {
		definitionName = baseName
		isChangeEvent = true
	}
	if definition, ok := storage.StandardObjectDefinition(definitionName); ok {
		for _, field := range definition.Fields {
			if field.APIName == "" {
				continue
			}
			members.fields[normalizeName(field.APIName)] = typesys.MemberSymbol{
				Kind:      apexast.DeclarationField,
				Name:      field.APIName,
				Type:      semaApexTypeForStorageField(field),
				Modifiers: []string{"public", "static", semaSyntheticStandardSObjectFieldModifier},
			}
			if field.RelationshipName != "" && len(field.ReferenceTo) != 0 {
				relationshipType := "SObject"
				if len(field.ReferenceTo) == 1 {
					relationshipType = field.ReferenceTo[0]
				}
				members.fields[normalizeName(field.RelationshipName)] = typesys.MemberSymbol{
					Kind:      apexast.DeclarationField,
					Name:      field.RelationshipName,
					Type:      relationshipType,
					Modifiers: []string{"public", semaSyntheticStandardSObjectFieldModifier},
				}
			}
		}
	}
	if isChangeEvent {
		members.fields[normalizeName("ChangeEventHeader")] = typesys.MemberSymbol{
			Kind:      apexast.DeclarationField,
			Name:      "ChangeEventHeader",
			Type:      "EventBus.ChangeEventHeader",
			Modifiers: []string{"public", semaSyntheticStandardSObjectFieldModifier},
		}
	}
	for _, field := range semaFallbackStandardSObjectFields(objectName, synthetic) {
		members.fields[normalizeName(field.Name)] = typesys.MemberSymbol{
			Kind:      apexast.DeclarationField,
			Name:      field.Name,
			Type:      semaApexTypeForSchemaField(field),
			Modifiers: []string{"public", "static", semaSyntheticStandardSObjectFieldModifier},
		}
		if field.RelationshipName != "" && len(field.ReferenceTo) != 0 {
			relationshipType := "SObject"
			if len(field.ReferenceTo) == 1 {
				relationshipType = field.ReferenceTo[0]
			}
			members.fields[normalizeName(field.RelationshipName)] = typesys.MemberSymbol{
				Kind:      apexast.DeclarationField,
				Name:      field.RelationshipName,
				Type:      relationshipType,
				Modifiers: []string{"public", semaSyntheticStandardSObjectFieldModifier},
			}
		}
	}
	addFallbackStandardSObjectRelationshipMembers(members, objectName)
	members.standardSObjectFieldsLoaded = true
}

type semaStandardChildRelationshipMember struct {
	name string
	typ  string
}

var semaStandardChildRelationshipCache struct {
	once     sync.Once
	byParent map[string][]semaStandardChildRelationshipMember
}

func ensureSemaStandardChildRelationshipMembers() {
	semaStandardChildRelationshipCache.once.Do(func() {
		byParent := make(map[string][]semaStandardChildRelationshipMember)
		seen := make(map[string]bool)
		for _, childObject := range storage.KnownStandardObjectNames() {
			childName := childObject
			if canonical, ok := storage.ResolveKnownStandardObjectName(childObject); ok {
				childName = canonical
			}
			storage.VisitStandardObjectRelationships(childName, nil, func(relationship storage.Relationship) {
				if relationship.ChildRelationship == "" {
					return
				}
				for _, parent := range relationship.ParentObjects {
					parentName := parent
					if canonical, ok := storage.ResolveKnownStandardObjectName(parent); ok {
						parentName = canonical
					}
					parentKey := normalizeName(parentName)
					if parentKey == "" {
						continue
					}
					member := semaStandardChildRelationshipMember{
						name: relationship.ChildRelationship,
						typ:  "List<" + childName + ">",
					}
					dedupeKey := parentKey + "\x00" + normalizeName(member.name) + "\x00" + normalizeName(member.typ)
					if seen[dedupeKey] {
						continue
					}
					seen[dedupeKey] = true
					byParent[parentKey] = append(byParent[parentKey], member)
				}
			})
		}
		semaStandardChildRelationshipCache.byParent = byParent
	})
}

func semaStandardChildRelationshipKey(objectName string) string {
	key := normalizeName(objectName)
	if canonical, ok := storage.ResolveKnownStandardObjectName(objectName); ok {
		key = normalizeName(canonical)
	}
	return key
}

func semaStandardChildRelationshipMembers(objectName string) []semaStandardChildRelationshipMember {
	ensureSemaStandardChildRelationshipMembers()
	key := semaStandardChildRelationshipKey(objectName)
	return append([]semaStandardChildRelationshipMember(nil), semaStandardChildRelationshipCache.byParent[key]...)
}

func semaStandardChildRelationshipMemberForKey(objectName, fieldKey string) (typesys.MemberSymbol, bool) {
	ensureSemaStandardChildRelationshipMembers()
	key := semaStandardChildRelationshipKey(objectName)
	for _, relationship := range semaStandardChildRelationshipCache.byParent[key] {
		if normalizeName(relationship.name) != fieldKey {
			continue
		}
		return typesys.MemberSymbol{
			Kind:      apexast.DeclarationField,
			Name:      relationship.name,
			Type:      relationship.typ,
			Modifiers: []string{"public", semaSyntheticStandardSObjectFieldModifier},
		}, true
	}
	return typesys.MemberSymbol{}, false
}

func semaStandardSObjectNameForKey(key string) (string, bool) {
	ensureSemaStandardSObjectMembers()
	name, ok := semaStandardSObjectMembersCache.nameByKey[key]
	return name, ok
}

func semaStandardChangeEventNames(objectNames []string) []string {
	seen := make(map[string]bool, len(objectNames))
	out := make([]string, 0, len(objectNames))
	for _, objectName := range objectNames {
		if _, ok := semaChangeEventBaseObjectName(objectName); ok {
			continue
		}
		name := objectName + "ChangeEvent"
		key := normalizeName(name)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func semaChangeEventBaseObjectName(objectName string) (string, bool) {
	name := strings.TrimSpace(objectName)
	if strings.HasSuffix(name, "__ChangeEvent") {
		base := strings.TrimSuffix(name, "__ChangeEvent") + "__c"
		return base, base != "__c"
	}
	if strings.HasSuffix(name, "ChangeEvent") {
		base := strings.TrimSuffix(name, "ChangeEvent")
		return base, base != ""
	}
	return "", false
}

func semaApplyPlatformInterfaceOverlays(model map[string]typeMembers) {
	semaSetPlatformInterface(model, "Callable", []string{"System.Callable"}, []typesys.MemberSymbol{{
		Kind: apexast.DeclarationMethod,
		Name: "call",
		Type: "Object",
		Parameters: []apexast.Parameter{
			{Name: "action", Type: "String"},
			{Name: "args", Type: "Map<String,Object>"},
		},
	}})
	semaSetPlatformInterface(model, "StubProvider", []string{"System.StubProvider"}, []typesys.MemberSymbol{{
		Kind: apexast.DeclarationMethod,
		Name: "handleMethodCall",
		Type: "Object",
		Parameters: []apexast.Parameter{
			{Name: "stubbedObject", Type: "Object"},
			{Name: "stubbedMethodName", Type: "String"},
			{Name: "returnType", Type: "Type"},
			{Name: "listOfParamTypes", Type: "List<Type>"},
			{Name: "listOfParamNames", Type: "List<String>"},
			{Name: "listOfArgs", Type: "List<Object>"},
		},
	}})
	semaSetPlatformInterface(model, "HttpCalloutMock", []string{"System.HttpCalloutMock"}, []typesys.MemberSymbol{{
		Kind: apexast.DeclarationMethod,
		Name: "respond",
		Type: "HttpResponse",
		Parameters: []apexast.Parameter{
			{Name: "request", Type: "HttpRequest"},
		},
	}})
}

func semaSetPlatformInterface(model map[string]typeMembers, name string, aliases []string, methods []typesys.MemberSymbol) {
	members, ok := model[normalizeName(name)]
	if !ok {
		members = typeMembers{
			name:     name,
			shortKey: semaShortTypeKey(name),
			kind:     apexast.DeclarationInterface,
			methods:  make(map[string][]typesys.MemberSymbol),
			fields:   make(map[string]typesys.MemberSymbol),
		}
	}
	if members.methods == nil {
		members.methods = make(map[string][]typesys.MemberSymbol)
	}
	if members.fields == nil {
		members.fields = make(map[string]typesys.MemberSymbol)
	}
	if members.kind == "" {
		members.kind = apexast.DeclarationInterface
	}
	for _, method := range methods {
		key := normalizeName(method.Name)
		if len(members.methods[key]) == 0 {
			members.methods[key] = append(members.methods[key], method)
		}
	}
	model[normalizeName(name)] = members
	for _, alias := range aliases {
		if _, exists := model[normalizeName(alias)]; !exists {
			model[normalizeName(alias)] = members
		}
	}
}

func semaApplyPlatformFieldOverlays(model map[string]typeMembers) {
	semaSetPlatformField(model, "RestContext", "request", "RestRequest", true)
	semaSetPlatformField(model, "RestContext", "response", "RestResponse", true)
	for _, field := range []struct {
		name string
		typ  string
	}{
		{"headers", "Map<String,String>"},
		{"httpMethod", "String"},
		{"params", "Map<String,String>"},
		{"remoteAddress", "String"},
		{"requestBody", "Blob"},
		{"requestURI", "String"},
		{"resourcePath", "String"},
	} {
		semaSetPlatformField(model, "RestRequest", field.name, field.typ, false)
	}
	for _, field := range []struct {
		name string
		typ  string
	}{
		{"headers", "Map<String,String>"},
		{"responseBody", "Blob"},
		{"statusCode", "Integer"},
	} {
		semaSetPlatformField(model, "RestResponse", field.name, field.typ, false)
	}
}

func semaSetPlatformField(model map[string]typeMembers, typeName, fieldName, fieldType string, static bool) {
	key := normalizeName(typeName)
	members, ok := model[key]
	if !ok {
		members = typeMembers{name: typeName, shortKey: semaShortTypeKey(typeName), methods: make(map[string][]typesys.MemberSymbol), fields: make(map[string]typesys.MemberSymbol)}
	}
	if members.fields == nil {
		members.fields = make(map[string]typesys.MemberSymbol)
	}
	field := members.fields[normalizeName(fieldName)]
	field.Kind = apexast.DeclarationProperty
	field.Name = fieldName
	field.Type = fieldType
	field.Modifiers = semaWithModifier(field.Modifiers, "public")
	if static {
		field.Modifiers = semaWithModifier(field.Modifiers, "static")
	}
	members.fields[normalizeName(fieldName)] = field
	model[key] = members
}

func semaWithModifier(modifiers []string, modifier string) []string {
	for _, existing := range modifiers {
		if strings.EqualFold(existing, modifier) {
			return modifiers
		}
	}
	return append(modifiers, modifier)
}

func semaAdditionalStandardSObjectNames() []string {
	return []string{"ApexClass", "ApexPage", "CronJobDetail", "CronTrigger", "EntityDefinition", "EntityParticle", "FieldDefinition", "FlowDefinitionView", "Folder", "NamedCredential", "Note", "RecentlyViewed", "Report", "UserEntityAccess", "UserFieldAccess", "UserRecordAccess"}
}

func semaAddCommonSObjectFields(fields map[string]typesys.MemberSymbol) {
	fields[normalizeName("SObjectType")] = typesys.MemberSymbol{
		Kind:      apexast.DeclarationField,
		Name:      "SObjectType",
		Type:      "Schema.SObjectType",
		Modifiers: []string{"public", "static", semaSyntheticStandardSObjectFieldModifier},
	}
	for _, field := range []schema.Field{
		{Name: "Id", Type: "Id"},
		{Name: "Name", Type: "Text"},
		{Name: "IsDeleted", Type: "Checkbox"},
		{Name: "CreatedDate", Type: "Datetime"},
		{Name: "LastActivityDate", Type: "Date"},
		{Name: "LastModifiedDate", Type: "Datetime"},
		{Name: "SystemModstamp", Type: "Datetime"},
		{Name: "CreatedById", Type: "Lookup", ReferenceTo: []string{"User"}, RelationshipName: "CreatedBy"},
		{Name: "LastModifiedById", Type: "Lookup", ReferenceTo: []string{"User"}, RelationshipName: "LastModifiedBy"},
		{Name: "OwnerId", Type: "Lookup", ReferenceTo: []string{"User"}, RelationshipName: "Owner"},
	} {
		key := normalizeName(field.Name)
		if _, exists := fields[key]; !exists {
			fields[key] = typesys.MemberSymbol{
				Kind:      apexast.DeclarationField,
				Name:      field.Name,
				Type:      semaApexTypeForSchemaField(field),
				Modifiers: []string{"public", semaSyntheticStandardSObjectFieldModifier},
			}
		}
		if field.RelationshipName == "" || len(field.ReferenceTo) == 0 {
			continue
		}
		relationshipKey := normalizeName(field.RelationshipName)
		if _, exists := fields[relationshipKey]; exists {
			continue
		}
		relationshipType := "SObject"
		if len(field.ReferenceTo) == 1 {
			relationshipType = field.ReferenceTo[0]
		}
		fields[relationshipKey] = typesys.MemberSymbol{
			Kind:      apexast.DeclarationField,
			Name:      field.RelationshipName,
			Type:      relationshipType,
			Modifiers: []string{"public", semaSyntheticStandardSObjectFieldModifier},
		}
	}
}

func semaAddSObjectProviderMembers(members *typeMembers, provider semaSObjectFieldProvider, namespace string, objects []schema.Object) {
	if members == nil || provider == nil {
		return
	}
	if members.fields == nil {
		members.fields = make(map[string]typesys.MemberSymbol)
	}
	if _, exists := members.fields[normalizeName("SObjectType")]; !exists {
		members.fields[normalizeName("SObjectType")] = typesys.MemberSymbol{
			Kind:      apexast.DeclarationField,
			Name:      "SObjectType",
			Type:      "Schema.SObjectType",
			Modifiers: []string{"public", "static", semaSyntheticStandardSObjectFieldModifier},
		}
	}
	provider.visit(func(field schema.Field) {
		fieldType := semaApexTypeForSchemaFieldInObjects(objects, field)
		if strings.EqualFold(field.Name, "ChangeEventHeader") && strings.EqualFold(field.Type, "EventBus.ChangeEventHeader") {
			fieldType = "EventBus.ChangeEventHeader"
		}
		if fieldType != "" {
			semaAddSchemaFieldMemberIfAbsent(members.fields, namespace, typesys.MemberSymbol{
				Kind:      apexast.DeclarationField,
				Name:      field.Name,
				Type:      fieldType,
				Modifiers: []string{"public"},
			})
		}
		if strings.EqualFold(field.Type, "Location") {
			for _, componentName := range semaLocationComponentFieldNames(field.Name) {
				semaAddSchemaFieldMemberIfAbsent(members.fields, namespace, typesys.MemberSymbol{
					Kind:      apexast.DeclarationField,
					Name:      componentName,
					Type:      "Decimal",
					Modifiers: []string{"public"},
				})
			}
		}
		if len(field.ReferenceTo) == 0 {
			return
		}
		relationshipType := "SObject"
		if len(field.ReferenceTo) == 1 {
			relationshipType = field.ReferenceTo[0]
		}
		if field.RelationshipName != "" {
			semaAddSchemaParentRelationshipMember(members, namespace, typesys.MemberSymbol{
				Kind:      apexast.DeclarationField,
				Name:      field.RelationshipName,
				Type:      relationshipType,
				Modifiers: []string{"public"},
			})
		}
		if relationshipFieldName := semaParentRelationshipFieldName(field.Name); relationshipFieldName != "" {
			semaAddSchemaParentRelationshipMember(members, namespace, typesys.MemberSymbol{
				Kind:      apexast.DeclarationField,
				Name:      relationshipFieldName,
				Type:      relationshipType,
				Modifiers: []string{"public"},
			})
		}
	})
}

func semaObjectSupportsRecordTypeRelationship(object schema.Object) bool {
	if strings.HasSuffix(strings.ToLower(object.Name), "__c") {
		return true
	}
	if len(object.RecordTypes) > 0 {
		return true
	}
	for _, field := range object.Fields {
		if strings.EqualFold(field.Name, "RecordTypeId") || strings.EqualFold(field.RelationshipName, "RecordType") {
			return true
		}
	}
	return false
}

func semaFallbackStandardSObjectFields(objectName string, synthetic bool) []schema.Field {
	switch {
	case strings.EqualFold(objectName, "Name"):
		return []schema.Field{
			{Name: "Id", Type: "Id"},
			{Name: "Name", Type: "Text"},
			{Name: "Type", Type: "Text"},
		}
	case strings.EqualFold(objectName, "FlowDefinitionView"):
		return []schema.Field{
			{Name: "Id", Type: "Id"},
			{Name: "ActiveVersionId", Type: "Id"},
			{Name: "ApiVersionRuntime", Type: "Number"},
			{Name: "ApiName", Type: "Text"},
			{Name: "Description", Type: "LongTextArea"},
			{Name: "DurableId", Type: "Text"},
			{Name: "FlowDefinitionViewId", Type: "Id"},
			{Name: "Label", Type: "Text"},
			{Name: "LastModifiedBy", Type: "Text"},
			{Name: "LastModifiedDate", Type: "Datetime"},
			{Name: "ManageableState", Type: "Picklist"},
			{Name: "OverriddenById", Type: "Lookup", ReferenceTo: []string{"Schema.FlowDefinitionView"}, RelationshipName: "OverriddenBy"},
			{Name: "OverriddenFlowId", Type: "Lookup", ReferenceTo: []string{"Schema.FlowDefinitionView"}, RelationshipName: "OverriddenFlow"},
			{Name: "ProcessType", Type: "Text"},
			{Name: "RecordTriggerType", Type: "Text"},
			{Name: "SourceTemplateId", Type: "Lookup", ReferenceTo: []string{"Schema.FlowDefinitionView"}, RelationshipName: "SourceTemplate"},
			{Name: "TriggerOrder", Type: "Number"},
			{Name: "TriggerType", Type: "Text"},
			{Name: "TriggerObjectOrEventId", Type: "Lookup", ReferenceTo: []string{"EntityDefinition"}, RelationshipName: "TriggerObjectOrEvent"},
		}
	case strings.EqualFold(objectName, "Event"):
		return []schema.Field{
			{Name: "IsClosed", Type: "Checkbox"},
		}
	case strings.EqualFold(objectName, "AccountShare"):
		return semaStandardShareFallbackFields("Account", []string{"AccountAccessLevel", "CaseAccessLevel", "ContactAccessLevel", "OpportunityAccessLevel"})
	case strings.EqualFold(objectName, "CaseShare"):
		return semaStandardShareFallbackFields("Case", []string{"CaseAccessLevel"})
	case strings.EqualFold(objectName, "ContactShare"):
		return semaStandardShareFallbackFields("Contact", []string{"ContactAccessLevel"})
	case strings.EqualFold(objectName, "LeadShare"):
		return semaStandardShareFallbackFields("Lead", []string{"LeadAccessLevel"})
	case strings.EqualFold(objectName, "OpportunityShare"):
		return semaStandardShareFallbackFields("Opportunity", []string{"OpportunityAccessLevel"})
	case strings.EqualFold(objectName, "ApexClass"):
		return []schema.Field{
			{Name: "Id", Type: "Id"},
			{Name: "Name", Type: "Text"},
			{Name: "NamespacePrefix", Type: "Text"},
			{Name: "Body", Type: "LongTextArea"},
		}
	case strings.EqualFold(objectName, "CronJobDetail"):
		return []schema.Field{
			{Name: "Id", Type: "Id"},
			{Name: "Name", Type: "Text"},
		}
	case strings.EqualFold(objectName, "CronTrigger"):
		return []schema.Field{
			{Name: "Id", Type: "Id"},
			{Name: "CronExpression", Type: "Text"},
			{Name: "TimesTriggered", Type: "Number"},
			{Name: "NextFireTime", Type: "Datetime"},
			{Name: "CronJobDetailId", Type: "Id"},
		}
	case synthetic:
		return []schema.Field{
			{Name: "Id", Type: "Id"},
			{Name: "Name", Type: "Text"},
		}
	default:
		return nil
	}
}

func semaStandardShareFallbackFields(parentObject string, accessFields []string) []schema.Field {
	fields := []schema.Field{
		{Name: "Id", Type: "Id"},
		{Name: parentObject + "Id", Type: "Lookup", ReferenceTo: []string{parentObject}, RelationshipName: parentObject},
		{Name: "UserOrGroupId", Type: "Lookup", ReferenceTo: []string{"Name"}, RelationshipName: "UserOrGroup"},
		{Name: "RowCause", Type: "Picklist"},
	}
	for _, accessField := range accessFields {
		fields = append(fields, schema.Field{Name: accessField, Type: "Picklist"})
	}
	return fields
}

func addFallbackStandardSObjectRelationshipMembers(members *typeMembers, objectName string) {
	if !strings.EqualFold(objectName, "PermissionSet") {
		return
	}
	members.fields[normalizeName("Assignments")] = typesys.MemberSymbol{
		Kind:      apexast.DeclarationField,
		Name:      "Assignments",
		Type:      "List<PermissionSetAssignment>",
		Modifiers: []string{"public", semaSyntheticStandardSObjectFieldModifier},
	}
}

func semaApexTypeForSchemaField(field schema.Field) string {
	return semaApexTypeForSchemaFieldInObjects(nil, field)
}

func semaApexTypeForSchemaFieldInObjects(objects []schema.Object, field schema.Field) string {
	fieldType := normalizeName(field.Type)
	if fieldType == "any" || fieldType == "object" {
		fieldType = ""
	}
	if fieldType == "summary" {
		if summarizedType := semaApexTypeForSummarizedField(objects, field.SummarizedField); summarizedType != "" {
			return summarizedType
		}
	}
	switch fieldType {
	case "id":
		return "Id"
	case "checkbox", "boolean":
		return "Boolean"
	case "int", "integer":
		return "Integer"
	case "long":
		return "Long"
	case "decimal", "double", "currency", "percent", "number", "summary":
		return "Decimal"
	case "date":
		return "Date"
	case "datetime":
		return "Datetime"
	case "time":
		return "Time"
	case "base64", "blob":
		return "Blob"
	case "address":
		return "Address"
	case "location":
		return "Location"
	case "metadatarelationship":
		return "String"
	case "lookup", "masterdetail", "externallookup", "indirectlookup", "reference":
		return "Id"
	case "string", "text", "textarea", "longtextarea", "html", "encryptedtext", "email", "phone", "url", "picklist", "multipicklist", "multiselectpicklist", "combobox", "autonumber":
		return "String"
	}
	if field.Name != "" {
		key := normalizeName(field.Name)
		localKey := normalizeName(semaSchemaLocalAPIName(field.Name))
		switch {
		case key == "id" || strings.HasSuffix(key, "id"):
			return "Id"
		case strings.HasSuffix(key, "email"):
			return "String"
		case key == "isdeleted" || strings.HasPrefix(key, "is") || strings.HasPrefix(key, "has") || localKey == "mandatory__c":
			return "Boolean"
		case strings.HasSuffix(localKey, "datetime__c"):
			return "Datetime"
		case strings.HasSuffix(localKey, "date__c"):
			return "Date"
		case key == "name" || key == "developername" || key == "masterlabel" || key == "label" || key == "namespaceprefix" || key == "qualifiedapiname":
			return "String"
		case strings.Contains(key, "name") || strings.Contains(key, "class") || strings.HasSuffix(key, "type") ||
			strings.Contains(localKey, "name") || strings.Contains(localKey, "class") || strings.HasSuffix(localKey, "type__c"):
			return "String"
		}
	}
	if fieldType == "" {
		return ""
	}
	return "Object"
}

func semaApexTypeForSummarizedField(objects []schema.Object, summarizedField string) string {
	objectName, fieldName, ok := strings.Cut(strings.TrimSpace(summarizedField), ".")
	if !ok || objectName == "" || fieldName == "" {
		return ""
	}
	for _, object := range objects {
		if !semaSchemaAPINameEquivalent(object.Name, objectName) {
			continue
		}
		for _, field := range object.Fields {
			if semaSchemaAPINameEquivalent(field.Name, fieldName) {
				return semaApexTypeForSchemaFieldInObjects(objects, field)
			}
		}
	}
	return ""
}

func semaSchemaAPINameEquivalent(left, right string) bool {
	if strings.EqualFold(left, right) {
		return true
	}
	return strings.EqualFold(semaSchemaLocalAPIName(left), semaSchemaLocalAPIName(right))
}

func semaSchemaLocalAPIName(name string) string {
	name = strings.TrimSpace(name)
	if !semaIsCustomAPIName(name) || !semaHasNamespaceToken(name) {
		return name
	}
	first := strings.Index(name, "__")
	if first <= 0 || first+2 >= len(name) {
		return name
	}
	return name[first+2:]
}

func semaApexTypeForStorageField(field storage.Field) string {
	switch field.Type {
	case storage.FieldID:
		return "Id"
	case storage.FieldBoolean:
		return "Boolean"
	case storage.FieldInteger:
		return "Integer"
	case storage.FieldDecimal:
		return "Decimal"
	case storage.FieldDate:
		return "Date"
	case storage.FieldDateTime:
		return "Datetime"
	case storage.FieldAddress:
		return "Address"
	case storage.FieldBlob:
		return "Blob"
	case storage.FieldReference:
		return "Id"
	case storage.FieldString, storage.FieldPicklist, storage.FieldMultiPicklist:
		return "String"
	default:
		if strings.HasSuffix(normalizeName(field.APIName), "address") {
			return "Address"
		}
		return semaApexTypeForSchemaField(schema.Field{Name: field.APIName})
	}
}

func semaEnumValues(typ typesys.TypeSymbol) []string {
	return semaEnumValuesWithSources(typ, newSemaSources(nil, nil))
}

func semaEnumValuesWithSources(typ typesys.TypeSymbol, sources *semaSources) []string {
	if typ.File == "" || typ.Range.Start.Offset < 0 || typ.Range.End.Offset <= typ.Range.Start.Offset {
		return nil
	}
	source, ok := sources.rawForType(typ)
	if !ok {
		return nil
	}
	if typ.Range.End.Offset > len(source) {
		return nil
	}
	decl := source[typ.Range.Start.Offset:typ.Range.End.Offset]
	open := strings.IndexByte(decl, '{')
	close := strings.LastIndexByte(decl, '}')
	if open < 0 || close <= open {
		return nil
	}
	body := decl[open+1 : close]
	// Strip both line and block comments before splitting on commas
	body = stripComments(body)
	parts := strings.Split(body, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		if idx := strings.IndexFunc(value, func(r rune) bool {
			return !(r == '_' || (r >= '0' && r <= '9') || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z'))
		}); idx >= 0 {
			value = strings.TrimSpace(value[:idx])
		}
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func stripComments(source string) string {
	var sb strings.Builder
	for len(source) > 0 {
		// Find the next comment start
		lineIdx := strings.Index(source, "//")
		blockIdx := strings.Index(source, "/*")
		if lineIdx < 0 && blockIdx < 0 {
			sb.WriteString(source)
			break
		}
		// Determine which comment comes first
		idx := lineIdx
		if blockIdx >= 0 && (lineIdx < 0 || blockIdx < lineIdx) {
			idx = blockIdx
		}
		sb.WriteString(source[:idx])
		if idx == lineIdx {
			// Line comment: skip to end of line
			if nl := strings.IndexByte(source[idx:], '\n'); nl >= 0 {
				source = source[idx+nl:]
			} else {
				break
			}
		} else {
			// Block comment: skip to */
			if end := strings.Index(source[idx+2:], "*/"); end >= 0 {
				source = source[idx+2+end+2:]
			} else {
				break
			}
		}
	}
	return sb.String()
}

func shortNestedTypeName(typeName string) string {
	if idx := strings.LastIndexByte(typeName, '.'); idx >= 0 {
		return typeName[idx+1:]
	}
	return typeName
}

func resolveNestedTypeName(model *semaTypeMemberView, owner, typeName string) string {
	typeName = strings.TrimSpace(typeName)
	if typeName == "" {
		return typeName
	}
	if semaShouldPreserveExplicitPlatformType(typeName) {
		return typeName
	}
	if strings.Contains(typeName, ".") {
		if owner != "" {
			candidate := owner + "." + typeName
			if _, ok := model.lookup(normalizeName(candidate)); ok {
				return candidate
			}
		}
		ownerParts := strings.Split(owner, ".")
		for i := len(ownerParts) - 1; i > 0; i-- {
			candidate := strings.Join(append(append([]string{}, ownerParts[:i]...), typeName), ".")
			if _, ok := model.lookup(normalizeName(candidate)); ok {
				return candidate
			}
		}
		if _, ok := model.lookup(normalizeName(typeName)); ok {
			return typeName
		}
		return semaCanonicalPlatformAlias(typeName)
	}
	ownerParts := strings.Split(owner, ".")
	if len(ownerParts) > 0 && strings.EqualFold(ownerParts[0], typeName) {
		return typeName
	}
	if semaIsCustomAPIName(typeName) {
		if _, ok := model.lookup(normalizeName(typeName)); ok {
			return typeName
		}
	}
	if namespace := semaOwnerTypeNamespace(model, owner); namespace != "" {
		if namespaced, ok := semaProjectNamespacedAPIName(namespace, typeName); ok {
			if _, exists := model.lookup(normalizeName(namespaced)); exists {
				return namespaced
			}
		}
	}
	if owner != "" {
		candidate := owner + "." + typeName
		if _, ok := model.lookup(normalizeName(candidate)); ok {
			return candidate
		}
	}
	for i := len(ownerParts) - 1; i > 0; i-- {
		candidate := strings.Join(append(append([]string{}, ownerParts[:i]...), typeName), ".")
		if _, ok := model.lookup(normalizeName(candidate)); ok {
			return candidate
		}
	}
	for i := len(ownerParts) - 1; i > 0; i-- {
		enclosing := strings.Join(ownerParts[:i], ".")
		if resolved := resolveNestedTypeNameFromSuperclasses(model, enclosing, typeName); resolved != "" {
			return resolved
		}
	}
	if resolved := resolveNestedTypeNameFromSuperclasses(model, owner, typeName); resolved != "" {
		return resolved
	}
	return semaCanonicalPlatformAlias(typeName)
}

func resolveNestedTypeNameFromSuperclasses(model *semaTypeMemberView, owner, typeName string) string {
	seen := make(map[string]bool)
	for current := owner; current != ""; {
		key := normalizeName(current)
		if key == "" || seen[key] {
			break
		}
		seen[key] = true
		members, ok := model.lookup(key)
		if !ok || strings.TrimSpace(members.superClass) == "" {
			break
		}
		candidate := members.superClass + "." + typeName
		if _, ok := model.lookup(normalizeName(candidate)); ok {
			return candidate
		}
		current = members.superClass
	}
	return ""
}

func semaShouldPreserveExplicitPlatformType(typeName string) bool {
	if !semaExplicitPlatformQualifiedName(typeName) {
		return false
	}
	canonical := semaCanonicalPlatformAlias(typeName)
	return !strings.EqualFold(canonical, typeName)
}

func semaOwnerTypeNamespace(model *semaTypeMemberView, owner string) string {
	members, _, ok := semaLookupTypeMembers(model, owner)
	if !ok || strings.TrimSpace(members.namespace) == "" {
		return ""
	}
	return members.namespace
}

func resolveNestedTypeReference(model *semaTypeMemberView, owner, typeName string) string {
	base, args := semaGenericBaseAndArgs(typeName)
	if len(args) == 0 {
		return resolveNestedTypeName(model, owner, typeName)
	}
	resolvedArgs := make([]string, len(args))
	for i, arg := range args {
		resolvedArgs[i] = resolveNestedTypeReference(model, owner, arg)
	}
	return resolveNestedTypeName(model, owner, base) + "<" + strings.Join(resolvedArgs, ",") + ">"
}

func buildConstructability(index typesys.Index) map[string]typesys.TypeSymbol {
	out := make(map[string]typesys.TypeSymbol)
	for _, typ := range index.Types {
		if !typ.Dependency {
			out[normalizeName(typ.Name)] = typ
		}
		if typ.Namespace != "" {
			out[normalizeName(typ.Namespace+"."+typ.Name)] = typ
		}
	}
	return out
}
