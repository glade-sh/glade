package codeintel

import (
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/typesys"
)

type schemaUseCollector struct {
	graph              Graph
	objects            map[string]schema.Object
	fields             map[string]map[string]schema.Field
	sortedObjectList   []schema.Object
	localObjectRegexes []schemaObjectRegexes
	assignmentRegexes  map[string]*regexp.Regexp
	triggerRegexes     map[string]*regexp.Regexp
}

type schemaObjectRegexes struct {
	object schema.Object
	local  *regexp.Regexp
	list   *regexp.Regexp
}

var (
	constructionRe     = regexp.MustCompile(`(?is)\bnew\s+([A-Za-z_][A-Za-z0-9_]*)\s*\((.*?)\)`)
	initializerFieldRe = regexp.MustCompile(`(?i)\b([A-Za-z_][A-Za-z0-9_]*)\s*=`)
	fieldAssignmentRe  = regexp.MustCompile(`(?i)\b([A-Za-z_][A-Za-z0-9_]*)\s*\.\s*([A-Za-z_][A-Za-z0-9_]*)\s*=`)
	fieldReadRe        = regexp.MustCompile(`(?i)\b([A-Za-z_][A-Za-z0-9_]*)\s*\.\s*([A-Za-z_][A-Za-z0-9_]*)\b`)
	putWriteRe         = regexp.MustCompile(`(?i)\b([A-Za-z_][A-Za-z0-9_]*)\s*\.\s*put\s*\(\s*'([^']+)'`)
	dmlRe              = regexp.MustCompile(`(?i)\b(insert|update|upsert|delete|undelete)\s+([A-Za-z_][A-Za-z0-9_]*)\b`)
	schemaFieldTokenRe = regexp.MustCompile(`(?i)\bSchema\s*\.\s*SObjectType\s*\.\s*([A-Za-z_][A-Za-z0-9_]*)\s*\.\s*fields\s*\.\s*([A-Za-z_][A-Za-z0-9_]*)`)
	schemaTypeTokenRe  = regexp.MustCompile(`(?i)\bSchema\s*\.\s*SObjectType\s*\.\s*([A-Za-z_][A-Za-z0-9_]*)\b`)
	sobjectTypeTokenRe = regexp.MustCompile(`(?i)\b([A-Za-z_][A-Za-z0-9_]*)\s*\.\s*SObjectType\b`)
)

func BuildSchemaUses(index typesys.Index) Graph {
	collector := newSchemaUseCollector(index)
	for _, path := range apexUseFiles(index) {
		collector.collectFile(index.Project.Root, path)
	}
	for _, trigger := range index.Triggers {
		collector.addTriggerObjectUse(index.Project.Root, trigger)
	}
	sortUses(collector.graph.Uses)
	return collector.graph
}

func newSchemaUseCollector(index typesys.Index) *schemaUseCollector {
	graph := BuildDeclarations(index)
	collector := &schemaUseCollector{
		graph:             graph,
		objects:           make(map[string]schema.Object),
		fields:            make(map[string]map[string]schema.Field),
		assignmentRegexes: make(map[string]*regexp.Regexp),
		triggerRegexes:    make(map[string]*regexp.Regexp),
	}
	for _, object := range index.Objects {
		collector.objects[strings.ToLower(object.Name)] = object
		fieldMap := make(map[string]schema.Field, len(object.Fields))
		for _, field := range object.Fields {
			fieldMap[strings.ToLower(field.Name)] = field
		}
		collector.fields[strings.ToLower(object.Name)] = fieldMap
	}
	collector.sortedObjectList = collector.sortedObjects()
	for _, object := range collector.sortedObjectList {
		quoted := regexp.QuoteMeta(object.Name)
		collector.localObjectRegexes = append(collector.localObjectRegexes, schemaObjectRegexes{
			object: object,
			local:  regexp.MustCompile(`(?i)\b` + quoted + `\s+([A-Za-z_][A-Za-z0-9_]*)`),
			list:   regexp.MustCompile(`(?i)\bList\s*<\s*` + quoted + `\s*>\s+([A-Za-z_][A-Za-z0-9_]*)`),
		})
	}
	return collector
}

func (c *schemaUseCollector) collectFile(root, path string) {
	data, err := os.ReadFile(sourcePath(root, path))
	if err != nil {
		return
	}
	source := string(data)
	locator := newSourceLocator(source)
	spans := newCodeSpans(source)
	codeSource := spans.maskNonCode(source)
	varTypes := c.collectLocalSObjectTypes(path, source, codeSource, locator)
	c.collectConstructions(path, source, codeSource, locator, varTypes)
	c.collectFieldAssignments(path, source, codeSource, locator, varTypes)
	c.collectFieldReads(path, source, codeSource, locator, varTypes)
	c.collectPutWrites(path, source, locator, spans, varTypes)
	c.collectDML(path, source, codeSource, locator, varTypes)
	c.collectSchemaTokens(path, source, codeSource, locator)
	c.collectSOQL(path, source, locator, spans)
}

func (c *schemaUseCollector) collectLocalSObjectTypes(path, source, codeSource string, locator sourceLocator) map[string]string {
	varTypes := make(map[string]string)
	for _, patterns := range c.localObjectRegexes {
		object := patterns.object
		for _, match := range patterns.local.FindAllStringSubmatchIndex(codeSource, -1) {
			name := source[match[2]:match[3]]
			if isApexKeyword(name) {
				continue
			}
			varTypes[strings.ToLower(name)] = object.Name
			c.addObjectUse(object.Name, UseRead, object.Name, path, locator.rangeFor(match[0], match[0]+len(object.Name)), nil)
		}
		for _, match := range patterns.list.FindAllStringSubmatchIndex(codeSource, -1) {
			name := source[match[2]:match[3]]
			varTypes[strings.ToLower(name)] = object.Name
		}
	}
	return varTypes
}

func (c *schemaUseCollector) collectConstructions(path, source, codeSource string, locator sourceLocator, varTypes map[string]string) {
	for _, match := range constructionRe.FindAllStringSubmatchIndex(codeSource, -1) {
		objectName := source[match[2]:match[3]]
		object, ok := c.object(objectName)
		if !ok {
			continue
		}
		c.addObjectUse(object.Name, UseConstruct, object.Name, path, locator.rangeFor(match[2], match[3]), nil)
		argsStart, argsEnd := match[4], match[5]
		c.collectInitializerFields(object.Name, path, source, codeSource, locator, argsStart, argsEnd)
		prefixStart := max(0, match[0]-160)
		if prefix := c.assignmentRegex(object.Name).FindStringSubmatch(codeSource[prefixStart:match[1]]); len(prefix) == 2 {
			varTypes[strings.ToLower(prefix[1])] = object.Name
		}
	}
}

func (c *schemaUseCollector) collectInitializerFields(objectName, path, source, codeSource string, locator sourceLocator, start, end int) {
	for _, match := range initializerFieldRe.FindAllStringSubmatchIndex(codeSource[start:end], -1) {
		fieldName := source[start+match[2] : start+match[3]]
		if field, ok := c.field(objectName, fieldName); ok {
			c.addFieldUse(objectName, field.Name, UseWrite, field.Name, path, locator.rangeFor(start+match[2], start+match[3]), nil)
		}
	}
}

func (c *schemaUseCollector) collectFieldAssignments(path, source, codeSource string, locator sourceLocator, varTypes map[string]string) {
	for _, match := range fieldAssignmentRe.FindAllStringSubmatchIndex(codeSource, -1) {
		varName := source[match[2]:match[3]]
		objectName, ok := varTypes[strings.ToLower(varName)]
		if !ok {
			continue
		}
		fieldName := source[match[4]:match[5]]
		if field, ok := c.field(objectName, fieldName); ok {
			c.addFieldUse(objectName, field.Name, UseWrite, field.Name, path, locator.rangeFor(match[4], match[5]), nil)
		}
	}
}

func (c *schemaUseCollector) collectFieldReads(path, source, codeSource string, locator sourceLocator, varTypes map[string]string) {
	for _, match := range fieldReadRe.FindAllStringSubmatchIndex(codeSource, -1) {
		if isFieldWriteTarget(codeSource, match[5]) {
			continue
		}
		varName := source[match[2]:match[3]]
		objectName, ok := varTypes[strings.ToLower(varName)]
		if !ok {
			continue
		}
		fieldName := source[match[4]:match[5]]
		if field, ok := c.field(objectName, fieldName); ok {
			c.addFieldUse(objectName, field.Name, UseRead, field.Name, path, locator.rangeFor(match[4], match[5]), nil)
		}
	}
}

func (c *schemaUseCollector) collectPutWrites(path, source string, locator sourceLocator, spans codeSpans, varTypes map[string]string) {
	for _, match := range putWriteRe.FindAllStringSubmatchIndex(source, -1) {
		if !spans.contains(match[0]) {
			continue
		}
		varName := source[match[2]:match[3]]
		objectName, ok := varTypes[strings.ToLower(varName)]
		if !ok {
			continue
		}
		fieldName := source[match[4]:match[5]]
		if field, ok := c.field(objectName, fieldName); ok {
			c.addFieldUse(objectName, field.Name, UseWrite, field.Name, path, locator.rangeFor(match[4], match[5]), nil)
		}
	}
}

func (c *schemaUseCollector) collectDML(path, source, codeSource string, locator sourceLocator, varTypes map[string]string) {
	for _, match := range dmlRe.FindAllStringSubmatchIndex(codeSource, -1) {
		varName := source[match[4]:match[5]]
		objectName, ok := varTypes[strings.ToLower(varName)]
		if !ok {
			continue
		}
		c.addObjectUse(objectName, UseMutate, objectName, path, locator.rangeFor(match[4], match[5]), map[string]string{
			"operation": strings.ToLower(source[match[2]:match[3]]),
		})
	}
}

func (c *schemaUseCollector) collectSchemaTokens(path, source, codeSource string, locator sourceLocator) {
	for _, match := range schemaFieldTokenRe.FindAllStringSubmatchIndex(codeSource, -1) {
		objectName := source[match[2]:match[3]]
		object, ok := c.object(objectName)
		if !ok {
			continue
		}
		c.addObjectUse(object.Name, UseMetadata, object.Name, path, locator.rangeFor(match[2], match[3]), map[string]string{"source": "schema_token"})
		fieldName := source[match[4]:match[5]]
		if field, ok := c.field(object.Name, fieldName); ok {
			c.addFieldUse(object.Name, field.Name, UseMetadata, field.Name, path, locator.rangeFor(match[4], match[5]), map[string]string{"source": "schema_token"})
		}
	}

	for _, match := range schemaTypeTokenRe.FindAllStringSubmatchIndex(codeSource, -1) {
		if followsSchemaFields(codeSource, match[3]) {
			continue
		}
		objectName := source[match[2]:match[3]]
		object, ok := c.object(objectName)
		if !ok {
			continue
		}
		c.addObjectUse(object.Name, UseMetadata, object.Name, path, locator.rangeFor(match[2], match[3]), map[string]string{"source": "schema_token"})
	}

	for _, match := range sobjectTypeTokenRe.FindAllStringSubmatchIndex(codeSource, -1) {
		objectName := source[match[2]:match[3]]
		object, ok := c.object(objectName)
		if !ok {
			continue
		}
		c.addObjectUse(object.Name, UseMetadata, object.Name, path, locator.rangeFor(match[2], match[3]), map[string]string{"source": "schema_token"})
	}
}

func (c *schemaUseCollector) collectSOQL(path, source string, locator sourceLocator, spans codeSpans) {
	for _, literal := range soqlLiterals(source, spans) {
		query, err := soql.Parse(literal.text)
		if err != nil {
			continue
		}
		c.collectSOQLQuery(query, query.Object, path, locator.rangeFor(literal.start, literal.end), nil)
	}
}

func (c *schemaUseCollector) collectSOQLQuery(query soql.Query, objectName, path string, queryRange diagnostic.Range, baseMetadata map[string]string) {
	object, ok := c.object(objectName)
	if !ok {
		return
	}
	c.addObjectUse(object.Name, UseQuery, object.Name, path, queryRange, withQueryPrecision(baseMetadata))
	for _, field := range query.Fields {
		c.addQueryFieldUse(object.Name, field, path, queryRange, baseMetadata)
	}
	for _, aggregate := range query.Aggregates {
		c.addQueryFieldUse(object.Name, aggregate.Field, path, queryRange, baseMetadata)
	}
	for _, field := range query.GroupBy {
		c.addQueryFieldUse(object.Name, field, path, queryRange, baseMetadata)
	}
	for _, order := range query.Order {
		c.addQueryFieldUse(object.Name, order.Field, path, queryRange, baseMetadata)
	}
	if query.Where != nil {
		c.collectSOQLCondition(object.Name, *query.Where, path, queryRange, baseMetadata)
	}
	for _, child := range query.ChildQueries {
		childObject, ok := c.childObjectForRelationship(object.Name, child.Relationship)
		if !ok {
			continue
		}
		metadata := copyMetadata(baseMetadata)
		metadata["parentObject"] = object.Name
		metadata["childRelationship"] = child.Relationship
		metadata["object"] = childObject.Name
		c.collectSOQLQuery(child.Query, childObject.Name, path, queryRange, metadata)
	}
	for _, spec := range query.Typeofs {
		for whenObject, fields := range spec.When {
			for _, field := range fields {
				metadata := copyMetadata(baseMetadata)
				metadata["relationshipPath"] = spec.Relationship + "." + field
				c.addQueryFieldUse(whenObject, field, path, queryRange, metadata)
			}
		}
		for _, field := range spec.Else {
			c.addQueryFieldUse(object.Name, spec.Relationship+"."+field, path, queryRange, baseMetadata)
		}
	}
}

func (c *schemaUseCollector) collectSOQLCondition(objectName string, condition soql.Condition, path string, queryRange diagnostic.Range, metadata map[string]string) {
	if condition.Field != "" {
		c.addQueryFieldUse(objectName, condition.Field, path, queryRange, metadata)
	}
	if condition.Subquery != nil {
		c.collectSOQLQuery(*condition.Subquery, condition.Subquery.Object, path, queryRange, metadata)
	}
	for _, child := range condition.And {
		c.collectSOQLCondition(objectName, child, path, queryRange, metadata)
	}
	for _, child := range condition.Or {
		c.collectSOQLCondition(objectName, child, path, queryRange, metadata)
	}
}

func (c *schemaUseCollector) addQueryFieldUse(objectName, fieldPath, path string, queryRange diagnostic.Range, metadata map[string]string) {
	if fieldPath == "" || fieldPath == "COUNT()" {
		return
	}
	if strings.Contains(fieldPath, "(") {
		return
	}
	if strings.Contains(fieldPath, ".") {
		c.addRelationshipFieldUse(objectName, fieldPath, path, queryRange, metadata)
		return
	}
	if field, ok := c.field(objectName, fieldPath); ok {
		c.addFieldUse(objectName, field.Name, UseQuery, field.Name, path, queryRange, withQueryPrecision(metadata))
	}
}

func (c *schemaUseCollector) addRelationshipFieldUse(rootObject, fieldPath, path string, queryRange diagnostic.Range, metadata map[string]string) {
	parts := strings.Split(fieldPath, ".")
	if len(parts) < 2 {
		return
	}
	currentObject, ok := c.object(rootObject)
	if !ok {
		return
	}
	for i, relationship := range parts[:len(parts)-1] {
		field, target, ok := c.relationshipField(currentObject.Name, relationship)
		if !ok {
			return
		}
		if i == len(parts)-2 {
			targetField, ok := c.field(target, parts[len(parts)-1])
			if !ok {
				return
			}
			useMetadata := copyMetadata(metadata)
			useMetadata["rangePrecision"] = "query"
			useMetadata["rootObject"] = rootObject
			useMetadata["relationship"] = relationship
			useMetadata["relationshipField"] = field.Name
			useMetadata["relationshipObject"] = target
			useMetadata["relationshipPath"] = fieldPath
			c.addFieldUse(target, targetField.Name, UseQuery, fieldPath, path, queryRange, useMetadata)
			return
		}
		next, ok := c.object(target)
		if !ok {
			return
		}
		currentObject = next
	}
}

func (c *schemaUseCollector) addTriggerObjectUse(root string, trigger typesys.TriggerSymbol) {
	if trigger.ObjectName == "" || trigger.File == "" {
		return
	}
	data, err := os.ReadFile(sourcePath(root, trigger.File))
	if err != nil {
		return
	}
	source := string(data)
	locator := newSourceLocator(source)
	if match := c.triggerRegex(trigger.ObjectName).FindStringIndex(source); match != nil {
		start := match[1] - len(trigger.ObjectName)
		c.addObjectUse(trigger.ObjectName, UseMetadata, trigger.ObjectName, trigger.File, locator.rangeFor(start, match[1]), map[string]string{
			"source":  "trigger",
			"trigger": trigger.Name,
		})
		return
	}
	c.addObjectUse(trigger.ObjectName, UseMetadata, trigger.ObjectName, trigger.File, trigger.Range, map[string]string{
		"source":  "trigger",
		"trigger": trigger.Name,
	})
}

func (c *schemaUseCollector) sortedObjects() []schema.Object {
	if c.sortedObjectList != nil {
		out := make([]schema.Object, len(c.sortedObjectList))
		copy(out, c.sortedObjectList)
		return out
	}
	out := make([]schema.Object, 0, len(c.objects))
	for _, object := range c.objects {
		out = append(out, object)
	}
	sort.Slice(out, func(i, j int) bool {
		return len(out[i].Name) > len(out[j].Name)
	})
	return out
}

func (c *schemaUseCollector) assignmentRegex(objectName string) *regexp.Regexp {
	key := strings.ToLower(objectName)
	if re := c.assignmentRegexes[key]; re != nil {
		return re
	}
	quoted := regexp.QuoteMeta(objectName)
	re := regexp.MustCompile(`(?i)\b` + quoted + `\s+([A-Za-z_][A-Za-z0-9_]*)\s*=\s*new\s+` + quoted + `\b`)
	c.assignmentRegexes[key] = re
	return re
}

func (c *schemaUseCollector) triggerRegex(objectName string) *regexp.Regexp {
	key := strings.ToLower(objectName)
	if re := c.triggerRegexes[key]; re != nil {
		return re
	}
	re := regexp.MustCompile(`(?i)\bon\s+` + regexp.QuoteMeta(objectName) + `\b`)
	c.triggerRegexes[key] = re
	return re
}

func (c *schemaUseCollector) object(name string) (schema.Object, bool) {
	object, ok := c.objects[strings.ToLower(name)]
	return object, ok
}

func (c *schemaUseCollector) field(objectName, fieldName string) (schema.Field, bool) {
	fields, ok := c.fields[strings.ToLower(objectName)]
	if !ok {
		return schema.Field{}, false
	}
	field, ok := fields[strings.ToLower(fieldName)]
	return field, ok
}

func (c *schemaUseCollector) relationshipField(objectName, relationshipName string) (schema.Field, string, bool) {
	fields, ok := c.fields[strings.ToLower(objectName)]
	if !ok {
		return schema.Field{}, "", false
	}
	for _, field := range fields {
		if strings.EqualFold(field.RelationshipName, relationshipName) && len(field.ReferenceTo) > 0 {
			return field, field.ReferenceTo[0], true
		}
	}
	return schema.Field{}, "", false
}

func (c *schemaUseCollector) childObjectForRelationship(parentObject, relationship string) (schema.Object, bool) {
	for _, object := range c.objects {
		for _, field := range object.Fields {
			if !strings.EqualFold(field.ChildRelationshipName, relationship) {
				continue
			}
			for _, target := range field.ReferenceTo {
				if strings.EqualFold(target, parentObject) {
					return object, true
				}
			}
		}
	}
	return schema.Object{}, false
}

func (c *schemaUseCollector) addObjectUse(objectName string, kind UseKind, name, path string, rng diagnostic.Range, metadata map[string]string) {
	object, ok := c.object(objectName)
	if !ok {
		return
	}
	c.graph.AddUse(Use{
		SymbolID: SObjectID(object.Name),
		Kind:     kind,
		Name:     name,
		File:     path,
		Range:    rng,
		Resolved: true,
		Metadata: metadata,
	})
}

func (c *schemaUseCollector) addFieldUse(objectName, fieldName string, kind UseKind, name, path string, rng diagnostic.Range, metadata map[string]string) {
	field, ok := c.field(objectName, fieldName)
	if !ok {
		return
	}
	object, ok := c.object(objectName)
	if !ok {
		return
	}
	c.graph.AddUse(Use{
		SymbolID: SObjectFieldID(object.Name, field.Name),
		Kind:     kind,
		Name:     name,
		File:     path,
		Range:    rng,
		Resolved: true,
		Metadata: metadata,
	})
}

type soqlLiteral struct {
	text       string
	start, end int
}

func soqlLiterals(source string, spans codeSpans) []soqlLiteral {
	var out []soqlLiteral
	for i := 0; i < len(source); i++ {
		if source[i] != '[' {
			continue
		}
		if !spans.contains(i) {
			continue
		}
		end := matchingBracket(source, i)
		if end == -1 {
			continue
		}
		text := strings.TrimSpace(source[i+1 : end])
		if strings.HasPrefix(strings.ToUpper(text), "SELECT ") {
			out = append(out, soqlLiteral{text: text, start: i, end: end + 1})
		}
		i = end
	}
	return out
}

type codeSpans []bool

func newCodeSpans(source string) codeSpans {
	spans := make(codeSpans, len(source))
	for i := range spans {
		spans[i] = true
	}
	for i := 0; i < len(source); i++ {
		if source[i] == '/' && i+1 < len(source) {
			switch source[i+1] {
			case '/':
				spans[i] = false
				spans[i+1] = false
				i += 2
				for i < len(source) && source[i] != '\n' {
					spans[i] = false
					i++
				}
				i--
				continue
			case '*':
				spans[i] = false
				spans[i+1] = false
				i += 2
				for i < len(source) {
					spans[i] = false
					if source[i] == '*' && i+1 < len(source) && source[i+1] == '/' {
						spans[i+1] = false
						i++
						break
					}
					i++
				}
				continue
			}
		}
		if source[i] == '\'' || source[i] == '"' {
			quote := source[i]
			spans[i] = false
			i++
			for i < len(source) {
				spans[i] = false
				if source[i] == '\\' {
					i++
					if i < len(source) {
						spans[i] = false
					}
					i++
					continue
				}
				if source[i] == quote {
					break
				}
				i++
			}
		}
	}
	return spans
}

func (s codeSpans) contains(offset int) bool {
	return offset >= 0 && offset < len(s) && s[offset]
}

func (s codeSpans) maskNonCode(source string) string {
	var b strings.Builder
	b.Grow(len(source))
	for i := 0; i < len(source); i++ {
		if s.contains(i) || source[i] == '\n' || source[i] == '\r' {
			b.WriteByte(source[i])
			continue
		}
		b.WriteByte(' ')
	}
	return b.String()
}

func matchingBracket(source string, start int) int {
	quote := byte(0)
	for i := start + 1; i < len(source); i++ {
		ch := source[i]
		if quote != 0 {
			if ch == '\\' {
				i++
				continue
			}
			if ch == quote {
				quote = 0
			}
			continue
		}
		if ch == '\'' || ch == '"' {
			quote = ch
			continue
		}
		if ch == ']' {
			return i
		}
	}
	return -1
}

type sourceLocator struct {
	source    string
	lineStart []int
}

func newSourceLocator(source string) sourceLocator {
	starts := []int{0}
	for offset, ch := range source {
		if ch == '\n' {
			starts = append(starts, offset+1)
		}
	}
	return sourceLocator{source: source, lineStart: starts}
}

func (l sourceLocator) rangeFor(start, end int) diagnostic.Range {
	return diagnostic.Range{
		Start: l.position(start),
		End:   l.position(end),
	}
}

func (l sourceLocator) position(offset int) diagnostic.Position {
	if offset < 0 {
		offset = 0
	}
	if offset > len(l.source) {
		offset = len(l.source)
	}
	line := sort.Search(len(l.lineStart), func(i int) bool {
		return l.lineStart[i] > offset
	})
	if line == 0 {
		line = 1
	}
	lineStart := l.lineStart[line-1]
	column := 1
	for range l.source[lineStart:offset] {
		column++
	}
	return diagnostic.Position{Line: line, Column: column, Offset: offset}
}

func withQueryPrecision(metadata map[string]string) map[string]string {
	out := copyMetadata(metadata)
	out["rangePrecision"] = "query"
	return out
}

func copyMetadata(metadata map[string]string) map[string]string {
	out := make(map[string]string, len(metadata)+1)
	for key, value := range metadata {
		out[key] = value
	}
	return out
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func isApexKeyword(name string) bool {
	switch strings.ToLower(name) {
	case "after", "and", "before", "by", "delete", "else", "false", "for", "from", "group", "if", "insert", "limit", "new", "null", "on", "order", "select", "true", "update", "upsert", "where":
		return true
	default:
		return false
	}
}

func isFieldWriteTarget(source string, offset int) bool {
	rest := strings.TrimLeft(source[offset:], " \t\r\n")
	for _, op := range []string{"<<=", ">>=", "+=", "-=", "*=", "/=", "%=", "&=", "|=", "^=", "="} {
		if !strings.HasPrefix(rest, op) {
			continue
		}
		if op == "=" && strings.HasPrefix(rest, "==") {
			return false
		}
		return true
	}
	return false
}

func followsSchemaFields(source string, offset int) bool {
	rest := strings.TrimLeft(source[offset:], " \t\r\n")
	return strings.HasPrefix(strings.ToLower(rest), ".fields")
}
