package apextest

import (
	"fmt"
	"hash/fnv"
	"os"
	"regexp"
	"strings"
	"sync"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

func compileProjectClasses(index typesys.Index, methods map[string]vm.Method, caches ...sourceCache) []vm.Class {
	var out []vm.Class
	sources := sourceCacheFor(caches)
	knownTypes := knownTypeNames(index.Types)
	methodsByClass := projectMethodsByClass(methods)
	for _, typ := range index.Types {
		if typ.Kind != apexast.DeclarationClass && typ.Kind != apexast.DeclarationInterface && typ.Kind != apexast.DeclarationEnum {
			continue
		}
		source, err := sources.read(typ.File)
		if err != nil {
			continue
		}
		class := vm.Class{
			Name:         typ.Name,
			Namespace:    typ.Namespace,
			Access:       accessModifier(typ.Modifiers),
			Modifiers:    append([]string(nil), typ.Modifiers...),
			IsAbstract:   hasModifier(typ.Modifiers, "abstract"),
			IsInterface:  typ.Kind == apexast.DeclarationInterface,
			IsTest:       typ.IsTest,
			Dependency:   typ.Dependency,
			Fields:       make(map[string]vm.Field),
			StaticFields: make(map[string]vm.Field),
			Methods:      make(map[string]vm.Method),
		}
		superClass := typ.SuperClass
		interfaces := append([]string(nil), typ.Interfaces...)
		typeSource, _ := typeDeclarationSource(source, typ.Range)
		if superClass == "" {
			superClass = parseExtends(typeSource)
		}
		if len(interfaces) == 0 {
			interfaces = parseImplements(typeSource)
		}
		class.SuperClass = qualifyNestedTypeName(typ.Name, superClass, knownTypes)
		class.Interfaces = qualifyNestedTypeNames(typ.Name, interfaces, knownTypes)
		if typ.Kind == apexast.DeclarationEnum {
			class.EnumValues = parseEnumValues(typeSource)
		}
		for _, method := range methodsByClass[projectMethodOwnerKey(typ.Name, typ.File)] {
			class.Methods[methodShortName(method.Name)+methodParamKey(method.Params)] = method
		}
		for _, member := range typ.Members {
			switch member.Kind {
			case apexast.DeclarationField, apexast.DeclarationProperty:
				field := vm.Field{
					Name:       member.Name,
					Type:       qualifyNestedTypeNameInType(typ.Name, member.Type, knownTypes),
					Static:     hasModifier(member.Modifiers, "static"),
					Access:     accessModifier(member.Modifiers),
					Modifiers:  append([]string(nil), member.Modifiers...),
					Property:   member.Kind == apexast.DeclarationProperty,
					File:       typ.File,
					Dependency: typ.Dependency,
				}
				if member.Kind == apexast.DeclarationProperty {
					attachPropertyAccessors(&field, typ.Name, typ.File, member, source)
				}
				if value, ok := compileFieldInitializer(member.Type, member.Name, member.Range, source); ok {
					field.Value = value
					field.InitialValue = value
				} else if initializer, ok := compileFieldInitializerMethod(typ.Name, field.Name, field.Static, typ.File, member.Range, source); ok {
					if field.Static {
						class.StaticInitializers = append(class.StaticInitializers, initializer)
					} else {
						class.InstanceInitializers = append(class.InstanceInitializers, initializer)
					}
				}
				if field.Static {
					class.StaticFields[field.Name] = field
					class.StaticFieldOrder = append(class.StaticFieldOrder, field.Name)
				} else {
					class.Fields[field.Name] = field
					class.FieldOrder = append(class.FieldOrder, field.Name)
				}
			case apexast.DeclarationConstructor:
				ctor, err := compileProjectConstructor(typ.Name, typ.File, member.Range, source)
				if err == nil {
					class.Constructors = append(class.Constructors, ctor)
				}
			case apexast.DeclarationInitializer:
				init, err := compileProjectInitializer(typ.Name, typ.File, member.Range, source, hasModifier(member.Modifiers, "static"))
				if err == nil {
					if init.IsStatic {
						class.StaticInitializers = append(class.StaticInitializers, init)
					} else {
						class.InstanceInitializers = append(class.InstanceInitializers, init)
					}
				}
			}
		}
		out = append(out, class)
	}
	out = append(out, passiveStandardRuntimeClasses(index.Types, out)...)
	return out
}
func compileProjectMethods(index typesys.Index, caches ...sourceCache) map[string]vm.Method {
	type methodCompileJob struct {
		ClassName  string
		Kind       apexast.DeclarationKind
		Member     typesys.MemberSymbol
		File       string
		Source     string
		APIVersion string
		Dependency bool
	}
	type methodCompileResult struct {
		Key    string
		Method vm.Method
	}
	sources := sourceCacheFor(caches)
	var jobs []methodCompileJob
	for _, typ := range index.Types {
		if typ.Kind != apexast.DeclarationClass && typ.Kind != apexast.DeclarationInterface {
			continue
		}
		source := ""
		sourceLoaded := false
		for _, member := range typ.Members {
			if member.Kind != apexast.DeclarationMethod || isTestSetup(member.Modifiers) || (member.IsTest && strings.EqualFold(member.Type, "void")) {
				continue
			}
			if !sourceLoaded {
				loaded, err := sources.read(typ.File)
				if err != nil {
					continue
				}
				source = loaded
				sourceLoaded = true
			}
			jobs = append(jobs, methodCompileJob{
				ClassName:  typ.Name,
				Kind:       typ.Kind,
				Member:     member,
				File:       typ.File,
				Source:     source,
				APIVersion: apiVersionForApexFile(typ.File),
				Dependency: typ.Dependency,
			})
		}
	}
	results := make([]methodCompileResult, len(jobs))
	compile := func(i int) {
		job := jobs[i]
		member := job.Member
		if job.Kind == apexast.DeclarationInterface {
			method, err := compileProjectMethodSignature(job.ClassName, member.Name, member.Type, append(member.Modifiers, "abstract"), job.File, member.Range, job.Source)
			if err == nil {
				method.Dependency = job.Dependency
				method.APIVersion = job.APIVersion
				results[i] = methodCompileResult{Key: projectMethodMapKey(method), Method: method}
			}
			return
		}
		method, err := compileProjectMethod(job.ClassName, member.Name, member.Type, member.Modifiers, job.File, member.Range, job.Source)
		if err != nil {
			if unsupported, ok := unsupportedProjectMethod(job.ClassName, member.Name, member.Type, member.Modifiers, job.File, member.Range, job.Source, err); ok {
				unsupported.Dependency = job.Dependency
				unsupported.APIVersion = job.APIVersion
				results[i] = methodCompileResult{Key: projectMethodMapKey(unsupported), Method: unsupported}
			}
			return
		}
		method.Dependency = job.Dependency
		method.APIVersion = job.APIVersion
		results[i] = methodCompileResult{Key: projectMethodMapKey(method), Method: method}
	}
	workers := compileWorkers(len(jobs))
	if workers <= 1 {
		for i := range jobs {
			compile(i)
		}
	} else {
		work := make(chan int)
		var wg sync.WaitGroup
		for i := 0; i < workers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for index := range work {
					compile(index)
				}
			}()
		}
		for i := range jobs {
			work <- i
		}
		close(work)
		wg.Wait()
	}

	out := make(map[string]vm.Method, len(results))
	for _, result := range results {
		if result.Key != "" {
			out[result.Key] = result.Method
		}
	}
	return out
}

var apexMetaAPIVersionPattern = regexp.MustCompile(`(?is)<apiVersion>\s*([0-9]+(?:\.[0-9]+)?)\s*</apiVersion>`)

func apiVersionForApexFile(file string) string {
	if strings.TrimSpace(file) == "" {
		return ""
	}
	data, err := os.ReadFile(file + "-meta.xml")
	if err != nil {
		return ""
	}
	match := apexMetaAPIVersionPattern.FindSubmatch(data)
	if len(match) != 2 {
		return ""
	}
	return string(match[1])
}

func compileProjectMethodSignature(className, methodName, returnType string, modifiers []string, file string, r diagnostic.Range, source string) (vm.Method, error) {
	methodSource, err := extractMethodSource(source, r)
	if err != nil {
		return vm.Method{}, err
	}
	params, err := parseParams(methodSource)
	if err != nil {
		return vm.Method{}, err
	}
	return vm.Method{
		Name:       className + "." + methodName,
		ReturnType: returnType,
		Params:     params,
		ClassName:  className,
		IsStatic:   hasModifier(modifiers, "static"),
		Access:     accessModifier(modifiers),
		Modifiers:  modifiers,
		File:       file,
		Line:       r.Start.Line,
		Column:     r.Start.Column,
	}, nil
}
func compileProjectTriggers(index typesys.Index, caches ...sourceCache) ([]vm.Trigger, []error) {
	var out []vm.Trigger
	var errs []error
	sources := sourceCacheFor(caches)
	for _, trigger := range index.Triggers {
		source, err := sources.read(trigger.File)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		body, err := extractMethodBody(source, trigger.Range)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		program, err := vm.CompileAnonymous(body)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		for _, event := range trigger.Events {
			timing, op := triggerEventParts(event)
			if timing == "" || op == "" {
				continue
			}
			out = append(out, vm.Trigger{
				Name:      trigger.Name,
				Namespace: trigger.Namespace,
				Object:    trigger.ObjectName,
				Timing:    timing,
				Operation: op,
				Program:   program,
				File:      trigger.File,
				Line:      trigger.Range.Start.Line,
				Column:    trigger.Range.Start.Column,
			})
		}
	}
	return out, errs
}
func applyProjectReferencedStandardFields(org *storage.OrgState, index typesys.Index, caches ...sourceCache) {
	if org == nil {
		return
	}
	childRelationshipLookup := projectReferencedChildRelationshipLookup(*org)
	cache := sourceCacheFor(caches)
	cacheKey := projectReferencedStandardFieldCacheKey(index, cache)
	if cacheKey != "" {
		if cached, ok := projectReferencedStandardFieldCache.Load(cacheKey); ok {
			fieldSet := cached.(projectReferencedStandardFieldSet)
			if len(fieldSet.Features) > 0 {
				storage.ApplyOrgShape(org, fieldSet.Features)
			}
			applyReferencedStandardFieldSet(org, fieldSet.Fields, childRelationshipLookup)
			return
		}
	}
	scan := parallelScanProjectReferencedStandardFields(org, index, cache, childRelationshipLookup)
	for objectName := range scan.CustomObjects {
		ensurePermissionReferencedObject(org, objectName)
	}
	for objectName := range scan.ListCustomSettings {
		recordProjectReferencedListCustomSetting(org, objectName)
	}
	inferred := scan.Inferred
	childRelationshipRefs := scan.ChildRelationshipRefs
	applyProjectReferencedSourceChildRelationships(org, inferred, childRelationshipRefs, childRelationshipLookup)
	features := projectReferencedOrgShapeFeatures(inferred)
	if len(features) > 0 {
		storage.ApplyOrgShape(org, features)
	}
	if cacheKey != "" {
		projectReferencedStandardFieldCache.Store(cacheKey, projectReferencedStandardFieldSet{Fields: inferred, Features: features})
	}
	applyReferencedStandardFieldSet(org, inferred, childRelationshipLookup)
}

func projectReferencedStandardFieldCacheKey(index typesys.Index, cache sourceCache) string {
	if index.Project.Root == "" {
		return ""
	}
	h := fnv.New64a()
	write := func(text string) {
		_, _ = h.Write([]byte(text))
		_, _ = h.Write([]byte{0})
	}
	write(index.Project.Root)
	write(index.Project.Namespace)
	write(fmt.Sprint(len(index.Types)))
	seenFiles := make(map[string]bool)
	for _, typ := range index.Types {
		if typ.File == "" || typ.Dependency || seenFiles[typ.File] {
			continue
		}
		seenFiles[typ.File] = true
		write(typ.File)
		source, err := cache.read(typ.File)
		if err != nil {
			write("read-error:" + err.Error())
			continue
		}
		write(source)
	}
	return fmt.Sprintf("%s|%016x", index.Project.Root, h.Sum64())
}

func projectReferencedOrgShapeFeatures(fields map[string]map[string]storage.Field) []string {
	accountFields := fields["Account"]
	if len(accountFields) == 0 {
		return nil
	}
	for fieldName := range accountFields {
		if projectReferencedAccountPersonField(fieldName) {
			return []string{"PersonAccounts"}
		}
	}
	return nil
}

func projectReferencedAccountPersonField(fieldName string) bool {
	fieldName = strings.TrimSpace(fieldName)
	return hasPrefixFold(fieldName, "Person") ||
		strings.EqualFold(fieldName, "FirstName") ||
		strings.EqualFold(fieldName, "LastName") ||
		strings.EqualFold(fieldName, "MiddleName") ||
		strings.EqualFold(fieldName, "Suffix") ||
		strings.EqualFold(fieldName, "Salutation") ||
		strings.EqualFold(fieldName, "IsPersonAccount")
}

func applyProjectReferencedRecordTypes(org *storage.OrgState, p project.Project, caches ...sourceCache) {
	if org == nil {
		return
	}
	for _, ref := range projectReferencedRecordTypes(p, caches...) {
		canonicalObject, ok := storage.ResolveObjectName(*org, ref.ObjectName)
		if !ok {
			continue
		}
		state := org.Objects[canonicalObject]
		if updateRecordTypeFromProjectReference(state.Definition.RecordTypes, ref.DeveloperName, ref.Name) {
			org.Objects[canonicalObject] = state
			continue
		}
		if profileRecordTypeExists(state.Definition.RecordTypes, ref.DeveloperName) || profileRecordTypeExists(state.Definition.RecordTypes, ref.Name) {
			continue
		}
		state.Definition.RecordTypes = append(state.Definition.RecordTypes, storage.RecordTypeInfo{
			DeveloperName: ref.DeveloperName,
			Name:          ref.Name,
			Active:        true,
			Available:     true,
		})
		org.Objects[canonicalObject] = state
	}
}

func applyManagedDependencyReferencedRecordTypes(org *storage.OrgState, p project.Project, caches ...sourceCache) {
	if org == nil {
		return
	}
	for _, dep := range p.ManagedPackageDependencies {
		if dep.Project == nil || dep.Status != "loaded" {
			continue
		}
		applyProjectReferencedRecordTypes(org, *dep.Project, caches...)
	}
}

func updateRecordTypeFromProjectReference(recordTypes []storage.RecordTypeInfo, developerName, name string) bool {
	for i := range recordTypes {
		recordType := &recordTypes[i]
		if !strings.EqualFold(recordType.DeveloperName, developerName) {
			continue
		}
		changed := false
		if strings.TrimSpace(name) != "" && (strings.TrimSpace(recordType.Name) == "" || !strings.EqualFold(name, developerName)) && !strings.EqualFold(recordType.Name, name) {
			recordType.Name = name
			changed = true
		}
		if !recordType.Active {
			recordType.Active = true
			changed = true
		}
		if !recordType.Available {
			recordType.Available = true
			changed = true
		}
		return changed
	}
	return false
}
func applyProjectDataRelationshipReferences(org *storage.OrgState, p project.Project) {
	if org == nil {
		return
	}
	for _, ref := range projectDataFieldReferences(p.Root) {
		ensureProjectDataReferencedObjectField(org, ref.ObjectName, ref.FieldName)
	}
	for _, ref := range projectDataRelationshipReferences(p.Root) {
		fieldName := dataRelationshipLookupFieldName(ref.ParentRelationship)
		if fieldName == "" {
			continue
		}
		ensurePermissionReferencedObjectField(org, ref.ChildObject, fieldName)
	}
}
func applyProjectProfileRecordTypes(org *storage.OrgState, p project.Project) bool {
	if org == nil || len(p.ProfileFiles) == 0 {
		return false
	}
	changed := false
	for _, file := range sortedProfileFilesForDefaults(p.ProfileFiles) {
		for _, visibility := range loadProfileRecordTypeVisibilities(file) {
			canonicalObject, ok := profileRecordTypeObjectName(*org, visibility.ObjectName)
			if !ok {
				continue
			}
			state := org.Objects[canonicalObject]
			if profileRecordTypeExists(state.Definition.RecordTypes, visibility.DeveloperName) {
				continue
			}
			state.Definition.RecordTypes = append(state.Definition.RecordTypes, storage.RecordTypeInfo{
				DeveloperName: visibility.DeveloperName,
				Name:          visibility.Name,
				Active:        true,
				Available:     true,
				Default:       visibility.Default && !visibility.PersonAccount,
			})
			org.Objects[canonicalObject] = state
			changed = true
		}
	}
	return changed
}
func applyProjectProfileRecordTypeDefaults(org *storage.OrgState, p project.Project) {
	if org == nil || len(p.ProfileFiles) == 0 {
		return
	}
	defaults := projectProfileRecordTypeDefaults(p.ProfileFiles)
	for objectName, developerName := range defaults {
		canonicalObject, ok := profileRecordTypeObjectName(*org, objectName)
		if !ok {
			continue
		}
		state := org.Objects[canonicalObject]
		changed := false
		for i := range state.Definition.RecordTypes {
			recordType := &state.Definition.RecordTypes[i]
			isDefault := strings.EqualFold(recordType.DeveloperName, developerName)
			if recordType.Default != isDefault {
				recordType.Default = isDefault
				changed = true
			}
		}
		if changed {
			org.Objects[canonicalObject] = state
		}
	}
}
func applyProjectProfileRecords(org *storage.OrgState, p project.Project, permissionSetMetadataCache map[string]permissionSetMetadataCacheEntry) {
	if org == nil || len(p.ProfileFiles) == 0 {
		return
	}
	state, ok := org.Objects["Profile"]
	if !ok {
		return
	}
	if state.Records == nil {
		state.Records = make(map[storage.ID]storage.Record)
	}
	if org.IDSequences == nil {
		org.IDSequences = make(map[string]uint64)
	}
	generator := storage.NewStandardIDGenerator()
	generator.Sequences = org.IDSequences
	for _, file := range p.ProfileFiles {
		name := profileNameFromPath(file)
		if name == "" || profileRecordExists(state, name) {
			continue
		}
		addProjectProfileRecord(org, &state, &generator, name)
	}
	if !profileRecordExists(state, "Customer Community User") {
		addProjectProfileRecord(org, &state, &generator, "Customer Community User")
	}
	objectState := org.Objects["ObjectPermissions"]
	fieldState := org.Objects["FieldPermissions"]
	objectPermissionKeys := objectPermissionRecordKeys(objectState)
	fieldPermissionKeys := fieldPermissionRecordKeys(fieldState)
	org.IDSequences = generator.Sequences
	org.Objects["Profile"] = state
	for _, file := range p.ProfileFiles {
		name := profileNameFromPath(file)
		if name == "" {
			continue
		}
		profileID, ok := recordFieldID(org.Objects["Profile"], "Name", name)
		if !ok {
			continue
		}
		applyProjectPermissionSetMetadataPermissions(org, file, string(profileID), &generator, objectPermissionKeys, fieldPermissionKeys, permissionSetMetadataCache)
	}
	org.IDSequences = generator.Sequences
}

func addProjectProfileRecord(org *storage.OrgState, state *storage.ObjectState, generator *storage.IDGenerator, name string) {
	id, err := generator.Next("Profile")
	if err != nil {
		return
	}
	fields := map[string]storage.Value{"Name": storage.StringValue(name)}
	if licenseID, ok := projectProfileLicenseID(org, name); ok {
		fields["UserLicenseId"] = storage.IDValue(licenseID)
	}
	state.Records[id] = storage.Record{
		ID:     id,
		Object: "Profile",
		Fields: fields,
	}
}

func projectProfileLicenseID(org *storage.OrgState, name string) (storage.ID, bool) {
	if org == nil {
		return "", false
	}
	lower := strings.ToLower(strings.TrimSpace(name))
	if !strings.Contains(lower, "community") && !strings.Contains(lower, "chatter external") {
		return "", false
	}
	state, ok := org.Objects["UserLicense"]
	if !ok {
		return "", false
	}
	for _, key := range []string{"PID_Customer_Community_Login", "PID_Customer_Community_Plus", "CSPLitePortal"} {
		if id, ok := recordFieldID(state, "LicenseDefinitionKey", key); ok {
			return id, true
		}
	}
	return "", false
}

func applyProjectPermissionSetRecords(org *storage.OrgState, p project.Project, permissionSetMetadataCache map[string]permissionSetMetadataCacheEntry) {
	if org == nil || len(p.PermissionSetFiles) == 0 {
		return
	}
	storage.EnsureStandardObject(org, "PermissionSet")
	state := org.Objects["PermissionSet"]
	if state.Records == nil {
		state.Records = make(map[storage.ID]storage.Record)
	}
	if org.IDSequences == nil {
		org.IDSequences = make(map[string]uint64)
	}
	generator := storage.NewStandardIDGenerator()
	generator.Sequences = org.IDSequences
	objectState := org.Objects["ObjectPermissions"]
	fieldState := org.Objects["FieldPermissions"]
	objectPermissionKeys := objectPermissionRecordKeys(objectState)
	fieldPermissionKeys := fieldPermissionRecordKeys(fieldState)
	for _, file := range p.PermissionSetFiles {
		name := metadataNameFromPath(file, ".permissionset-meta.xml", ".permissionset")
		if name == "" {
			continue
		}
		id, exists := recordFieldID(state, "Name", name)
		if !exists {
			nextID, err := generator.Next("PermissionSet")
			if err != nil {
				continue
			}
			id = nextID
			metadata, hasMetadata := readPermissionSetMetadata(file, permissionSetMetadataCache)
			label := strings.ReplaceAll(name, "_", " ")
			if hasMetadata && strings.TrimSpace(metadata.Label) != "" {
				label = strings.TrimSpace(metadata.Label)
			}
			state.Records[id] = storage.Record{
				ID:     id,
				Object: "PermissionSet",
				Fields: map[string]storage.Value{
					"Name":             storage.StringValue(name),
					"Label":            storage.StringValue(label),
					"Type":             storage.StringValue("Regular"),
					"IsOwnedByProfile": storage.BooleanValue(false),
				},
			}
		}
		metadata, metadataOK := readPermissionSetMetadata(file, permissionSetMetadataCache)
		if metadataOK {
			customPermissions := permissionSetCustomPermissionValues(metadata)
			if len(customPermissions) > 0 {
				record := state.Records[id]
				if record.Fields == nil {
					record.Fields = make(map[string]storage.Value)
				}
				record.Fields["CustomPermissions"] = storage.ListValue(customPermissions...)
				state.Records[id] = record
			}
		}
		applyProjectPermissionSetMetadataPermissions(org, file, string(id), &generator, objectPermissionKeys, fieldPermissionKeys, permissionSetMetadataCache)
		applyProjectGuestPermissionSetAssignment(org, name, id, &generator)
	}
	org.IDSequences = generator.Sequences
	org.Objects["PermissionSet"] = state
}

func applyProjectGuestPermissionSetAssignment(org *storage.OrgState, permissionSetName string, permissionSetID storage.ID, generator *storage.IDGenerator) {
	if org == nil || generator == nil || !permissionSetLooksGuestScoped(permissionSetName) {
		return
	}
	guestUserID, ok := ensureProjectGuestUser(org)
	if !ok {
		return
	}
	storage.EnsureStandardObject(org, "PermissionSetAssignment")
	state := org.Objects["PermissionSetAssignment"]
	if state.Records == nil {
		state.Records = make(map[storage.ID]storage.Record)
	}
	for _, record := range state.Records {
		assignee, hasAssignee := record.GetField("AssigneeId")
		permissionSet, hasPermissionSet := record.GetField("PermissionSetId")
		if hasAssignee && hasPermissionSet &&
			storageIDValueEqualsText(assignee, string(guestUserID)) &&
			storageIDValueEqualsText(permissionSet, string(permissionSetID)) {
			return
		}
	}
	id, err := generator.Next("PermissionSetAssignment")
	if err != nil {
		return
	}
	state.Records[id] = storage.Record{
		ID:     id,
		Object: "PermissionSetAssignment",
		Fields: map[string]storage.Value{
			"AssigneeId":      storage.IDValue(guestUserID),
			"PermissionSetId": storage.IDValue(permissionSetID),
		},
	}
	org.Objects["PermissionSetAssignment"] = state
}

func permissionSetLooksGuestScoped(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return false
	}
	return strings.Contains(name, "guest")
}

func projectGuestUserID(org *storage.OrgState) (storage.ID, bool) {
	if org == nil {
		return "", false
	}
	state, ok := org.Objects["User"]
	if !ok {
		return "", false
	}
	for _, record := range state.Records {
		if value, ok := record.GetField("UserType"); ok && strings.EqualFold(value.String, "Guest") {
			return record.ID, true
		}
	}
	return "", false
}

func ensureProjectGuestUser(org *storage.OrgState) (storage.ID, bool) {
	if id, ok := projectGuestUserID(org); ok {
		return id, true
	}
	if org == nil {
		return "", false
	}
	storage.EnsureStandardObject(org, "User")
	state := org.Objects["User"]
	if state.Records == nil {
		state.Records = make(map[storage.ID]storage.Record)
	}
	id := storage.ID("005000000000G01")
	if _, exists := state.Records[id]; !exists {
		state.Records[id] = storage.Record{
			ID:     id,
			Object: "User",
			Fields: map[string]storage.Value{
				"Username":          storage.StringValue("guest@example.invalid"),
				"Alias":             storage.StringValue("guest"),
				"Email":             storage.StringValue("guest@example.invalid"),
				"IsActive":          storage.BooleanValue(true),
				"UserType":          storage.StringValue("Guest"),
				"LocaleSidKey":      storage.StringValue("en_US"),
				"LanguageLocaleKey": storage.StringValue("en_US"),
				"TimeZoneSidKey":    storage.StringValue("UTC"),
				"EmailEncodingKey":  storage.StringValue("UTF-8"),
			},
		}
	}
	org.Objects["User"] = state
	return id, true
}

func applyProjectPermissionSetMetadataPermissions(org *storage.OrgState, file, parentID string, generator *storage.IDGenerator, objectPermissionKeys map[string]bool, fieldPermissionKeys map[string]bool, permissionSetMetadataCache map[string]permissionSetMetadataCacheEntry) {
	if org == nil || strings.TrimSpace(parentID) == "" || generator == nil {
		return
	}
	metadata, ok := readPermissionSetMetadata(file, permissionSetMetadataCache)
	if !ok {
		return
	}
	storage.EnsureStandardObject(org, "ObjectPermissions")
	storage.EnsureStandardObject(org, "FieldPermissions")
	objectState := org.Objects["ObjectPermissions"]
	if objectState.Records == nil {
		objectState.Records = make(map[storage.ID]storage.Record)
	}
	fieldState := org.Objects["FieldPermissions"]
	if fieldState.Records == nil {
		fieldState.Records = make(map[storage.ID]storage.Record)
	}
	for _, permission := range metadata.ObjectPermission {
		objectName := strings.TrimSpace(permission.Object)
		key := objectPermissionRecordKey(parentID, objectName)
		if key == "" || objectPermissionKeys[key] {
			continue
		}
		ensurePermissionReferencedObject(org, objectName)
		id, err := generator.Next("ObjectPermissions")
		if err != nil {
			continue
		}
		objectState.Records[id] = storage.Record{
			ID:     id,
			Object: "ObjectPermissions",
			Fields: map[string]storage.Value{
				"ParentId":                    storage.IDValue(storage.ID(parentID)),
				"SObjectType":                 storage.StringValue(objectName),
				"PermissionsRead":             storage.BooleanValue(permission.AllowRead),
				"PermissionsCreate":           storage.BooleanValue(permission.AllowCreate),
				"PermissionsEdit":             storage.BooleanValue(permission.AllowEdit),
				"PermissionsDelete":           storage.BooleanValue(permission.AllowDelete),
				"PermissionsViewAllRecords":   storage.BooleanValue(permission.ViewAllRecords),
				"PermissionsModifyAllRecords": storage.BooleanValue(permission.ModifyAllRecords),
			},
		}
		objectPermissionKeys[key] = true
	}
	for _, permission := range metadata.FieldPermissions {
		fieldName := strings.TrimSpace(permission.Field)
		if fieldName == "" {
			continue
		}
		objectName := fieldPermissionObjectName(fieldName)
		key := fieldPermissionRecordKey(parentID, objectName, fieldName)
		if objectName == "" || fieldPermissionKeys[key] {
			continue
		}
		ensurePermissionReferencedObjectField(org, objectName, fieldName)
		id, err := generator.Next("FieldPermissions")
		if err != nil {
			continue
		}
		fieldState.Records[id] = storage.Record{
			ID:     id,
			Object: "FieldPermissions",
			Fields: map[string]storage.Value{
				"ParentId":        storage.IDValue(storage.ID(parentID)),
				"SObjectType":     storage.StringValue(objectName),
				"Field":           storage.StringValue(fieldName),
				"PermissionsRead": storage.BooleanValue(permission.Readable),
				"PermissionsEdit": storage.BooleanValue(permission.Editable),
			},
		}
		fieldPermissionKeys[key] = true
	}
	for _, visibility := range metadata.RecordTypeVisibilities {
		if !visibility.Visible {
			continue
		}
		objectName, developerName, ok := strings.Cut(strings.TrimSpace(visibility.RecordType), ".")
		if !ok || objectName == "" || developerName == "" {
			continue
		}
		ensurePermissionReferencedObject(org, objectName)
		canonicalObject, ok := profileRecordTypeObjectName(*org, objectName)
		if !ok {
			continue
		}
		developerName = stripRecordTypeNamespaceToken(developerName)
		state := org.Objects[canonicalObject]
		if profileRecordTypeExists(state.Definition.RecordTypes, developerName) {
			continue
		}
		state.Definition.RecordTypes = append(state.Definition.RecordTypes, storage.RecordTypeInfo{
			DeveloperName: developerName,
			Name:          recordTypeLabelFromDeveloperName(developerName),
			Active:        true,
			Available:     true,
			Default:       visibility.Default,
		})
		org.Objects[canonicalObject] = state
	}
	org.Objects["ObjectPermissions"] = objectState
	org.Objects["FieldPermissions"] = fieldState
}
func applyProjectPermissionSetGroupRecords(org *storage.OrgState, p project.Project) {
	if org == nil || len(p.PermissionSetGroupFiles) == 0 {
		return
	}
	storage.EnsureStandardObject(org, "PermissionSetGroup")
	state := org.Objects["PermissionSetGroup"]
	if state.Records == nil {
		state.Records = make(map[storage.ID]storage.Record)
	}
	if org.IDSequences == nil {
		org.IDSequences = make(map[string]uint64)
	}
	generator := storage.NewStandardIDGenerator()
	generator.Sequences = org.IDSequences
	for _, file := range p.PermissionSetGroupFiles {
		name := metadataNameFromPath(file, ".permissionsetgroup-meta.xml", ".permissionsetgroup")
		if name == "" || recordFieldExists(state, "DeveloperName", name) {
			continue
		}
		id, err := generator.Next("PermissionSetGroup")
		if err != nil {
			continue
		}
		state.Records[id] = storage.Record{
			ID:     id,
			Object: "PermissionSetGroup",
			Fields: map[string]storage.Value{
				"DeveloperName": storage.StringValue(name),
				"MasterLabel":   storage.StringValue(strings.ReplaceAll(name, "_", " ")),
				"Status":        storage.StringValue("Updated"),
			},
		}
	}
	org.IDSequences = generator.Sequences
	org.Objects["PermissionSetGroup"] = state
}

func compileProjectMethod(className, methodName, returnType string, modifiers []string, file string, r diagnostic.Range, source string) (vm.Method, error) {
	methodSource, err := extractMethodSource(source, r)
	if err != nil {
		return vm.Method{}, err
	}
	params, err := parseParams(methodSource)
	if err != nil {
		return vm.Method{}, err
	}
	body, err := extractMethodBody(source, r)
	if err != nil {
		return vm.Method{}, err
	}
	program, err := vm.CompileAnonymous(body)
	if err != nil {
		return vm.Method{}, err
	}
	return vm.Method{
		Name:       className + "." + methodName,
		ReturnType: returnType,
		Params:     params,
		Program:    program,
		ClassName:  className,
		IsStatic:   hasModifier(modifiers, "static"),
		Access:     accessModifier(modifiers),
		Modifiers:  modifiers,
		File:       file,
		Line:       r.Start.Line,
		Column:     r.Start.Column,
	}, nil
}
func compileProjectConstructor(className, file string, r diagnostic.Range, source string) (vm.Method, error) {
	methodSource, err := extractMethodSource(source, r)
	if err != nil {
		return vm.Method{}, err
	}
	params, err := parseParams(methodSource)
	if err != nil {
		return vm.Method{}, err
	}
	body, err := extractMethodBody(source, r)
	if err != nil {
		return vm.Method{}, err
	}
	program, err := vm.CompileAnonymous(body)
	if err != nil {
		return vm.Method{}, err
	}
	return vm.Method{
		Name:          className + ".<init>",
		ReturnType:    "void",
		Params:        params,
		Program:       program,
		ClassName:     className,
		IsConstructor: true,
		File:          file,
		Line:          r.Start.Line,
		Column:        r.Start.Column,
	}, nil
}
func compileProjectInitializer(className, file string, r diagnostic.Range, source string, static bool) (vm.Method, error) {
	body, err := extractMethodBody(source, r)
	if err != nil {
		return vm.Method{}, err
	}
	program, err := vm.CompileAnonymous(body)
	if err != nil {
		return vm.Method{}, err
	}
	name := className + ".<init_block>"
	if static {
		name = className + ".<static_init>"
	}
	return vm.Method{
		Name:       name,
		ReturnType: "void",
		Program:    program,
		ClassName:  className,
		IsStatic:   static,
		File:       file,
		Line:       r.Start.Line,
		Column:     r.Start.Column,
	}, nil
}
