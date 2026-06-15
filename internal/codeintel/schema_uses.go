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
	graph   Graph
	objects map[string]schema.Object
	fields  map[string]map[string]schema.Field
}

func BuildSchemaUses(index typesys.Index) Graph {
	collector := newSchemaUseCollector(index)
	for _, path := range apexUseFiles(index) {
		collector.collectFile(path)
	}
	for _, trigger := range index.Triggers {
		collector.addTriggerObjectUse(trigger)
	}
	sortUses(collector.graph.Uses)
	return collector.graph
}

func newSchemaUseCollector(index typesys.Index) *schemaUseCollector {
	graph := BuildDeclarations(index)
	collector := &schemaUseCollector{
		graph:   graph,
		objects: make(map[string]schema.Object),
		fields:  make(map[string]map[string]schema.Field),
	}
	for _, object := range index.Objects {
		collector.objects[strings.ToLower(object.Name)] = object
		fieldMap := make(map[string]schema.Field, len(object.Fields))
		for _, field := range object.Fields {
			fieldMap[strings.ToLower(field.Name)] = field
		}
		collector.fields[strings.ToLower(object.Name)] = fieldMap
	}
	return collector
}

func apexUseFiles(index typesys.Index) []string {
	seen := make(map[string]struct{})
	for _, typ := range index.Types {
		if typ.File != "" {
			seen[typ.File] = struct{}{}
		}
	}
	for _, trigger := range index.Triggers {
		if trigger.File != "" {
			seen[trigger.File] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for path := range seen {
		out = append(out, path)
	}
	sort.Strings(out)
	return out
}

func (c *schemaUseCollector) collectFile(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	source := string(data)
	locator := newSourceLocator(source)
	varTypes := c.collectLocalSObjectTypes(path, source, locator)
	c.collectConstructions(path, source, locator, varTypes)
	c.collectFieldAssignments(path, source, locator, varTypes)
	c.collectPutWrites(path, source, locator, varTypes)
	c.collectDML(path, source, locator, varTypes)
	c.collectSchemaTokens(path, source, locator)
	c.collectSOQL(path, source, locator)
}

func (c *schemaUseCollector) collectLocalSObjectTypes(path, source string, locator sourceLocator) map[string]string {
	varTypes := make(map[string]string)
	for _, object := range c.sortedObjects() {
		re := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(object.Name) + `\s+([A-Za-z_][A-Za-z0-9_]*)`)
		for _, match := range re.FindAllStringSubmatchIndex(source, -1) {
			name := source[match[2]:match[3]]
			if isApexKeyword(name) {
				continue
			}
			varTypes[strings.ToLower(name)] = object.Name
			c.addObjectUse(object.Name, UseRead, object.Name, path, locator.rangeFor(match[0], match[0]+len(object.Name)), nil)
		}
		listRe := regexp.MustCompile(`(?i)\bList\s*<\s*` + regexp.QuoteMeta(object.Name) + `\s*>\s+([A-Za-z_][A-Za-z0-9_]*)`)
		for _, match := range listRe.FindAllStringSubmatchIndex(source, -1) {
			name := source[match[2]:match[3]]
			varTypes[strings.ToLower(name)] = object.Name
		}
	}
	return varTypes
}

func (c *schemaUseCollector) collectConstructions(path, source string, locator sourceLocator, varTypes map[string]string) {
	re := regexp.MustCompile(`(?is)\bnew\s+([A-Za-z_][A-Za-z0-9_]*)\s*\((.*?)\)`)
	for _, match := range re.FindAllStringSubmatchIndex(source, -1) {
		objectName := source[match[2]:match[3]]
		object, ok := c.object(objectName)
		if !ok {
			continue
		}
		c.addObjectUse(object.Name, UseConstruct, object.Name, path, locator.rangeFor(match[2], match[3]), nil)
		argsStart, argsEnd := match[4], match[5]
		c.collectInitializerFields(object.Name, path, source, locator, argsStart, argsEnd)
		assignmentRe := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(object.Name) + `\s+([A-Za-z_][A-Za-z0-9_]*)\s*=\s*new\s+` + regexp.QuoteMeta(object.Name) + `\b`)
		prefixStart := max(0, match[0]-160)
		if prefix := assignmentRe.FindStringSubmatch(source[prefixStart:match[1]]); len(prefix) == 2 {
			varTypes[strings.ToLower(prefix[1])] = object.Name
		}
	}
}

func (c *schemaUseCollector) collectInitializerFields(objectName, path, source string, locator sourceLocator, start, end int) {
	re := regexp.MustCompile(`(?i)\b([A-Za-z_][A-Za-z0-9_]*)\s*=`)
	for _, match := range re.FindAllStringSubmatchIndex(source[start:end], -1) {
		fieldName := source[start+match[2] : start+match[3]]
		if field, ok := c.field(objectName, fieldName); ok {
			c.addFieldUse(objectName, field.Name, UseWrite, field.Name, path, locator.rangeFor(start+match[2], start+match[3]), nil)
		}
	}
}

func (c *schemaUseCollector) collectFieldAssignments(path, source string, locator sourceLocator, varTypes map[string]string) {
	re := regexp.MustCompile(`(?i)\b([A-Za-z_][A-Za-z0-9_]*)\s*\.\s*([A-Za-z_][A-Za-z0-9_]*)\s*=`)
	for _, match := range re.FindAllStringSubmatchIndex(source, -1) {
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

func (c *schemaUseCollector) collectPutWrites(path, source string, locator sourceLocator, varTypes map[string]string) {
	re := regexp.MustCompile(`(?i)\b([A-Za-z_][A-Za-z0-9_]*)\s*\.\s*put\s*\(\s*'([^']+)'`)
	for _, match := range re.FindAllStringSubmatchIndex(source, -1) {
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

func (c *schemaUseCollector) collectDML(path, source string, locator sourceLocator, varTypes map[string]string) {
	re := regexp.MustCompile(`(?i)\b(insert|update|upsert|delete|undelete)\s+([A-Za-z_][A-Za-z0-9_]*)\b`)
	for _, match := range re.FindAllStringSubmatchIndex(source, -1) {
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

func (c *schemaUseCollector) collectSchemaTokens(path, source string, locator sourceLocator) {
	re := regexp.MustCompile(`(?i)\bSchema\s*\.\s*SObjectType\s*\.\s*([A-Za-z_][A-Za-z0-9_]*)\s*\.\s*fields\s*\.\s*([A-Za-z_][A-Za-z0-9_]*)`)
	for _, match := range re.FindAllStringSubmatchIndex(source, -1) {
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

	schemaTypeRe := regexp.MustCompile(`(?i)\bSchema\s*\.\s*SObjectType\s*\.\s*([A-Za-z_][A-Za-z0-9_]*)\b`)
	for _, match := range schemaTypeRe.FindAllStringSubmatchIndex(source, -1) {
		if followsSchemaFields(source, match[3]) {
			continue
		}
		objectName := source[match[2]:match[3]]
		object, ok := c.object(objectName)
		if !ok {
			continue
		}
		c.addObjectUse(object.Name, UseMetadata, object.Name, path, locator.rangeFor(match[2], match[3]), map[string]string{"source": "schema_token"})
	}

	sobjectTypeRe := regexp.MustCompile(`(?i)\b([A-Za-z_][A-Za-z0-9_]*)\s*\.\s*SObjectType\b`)
	for _, match := range sobjectTypeRe.FindAllStringSubmatchIndex(source, -1) {
		objectName := source[match[2]:match[3]]
		object, ok := c.object(objectName)
		if !ok {
			continue
		}
		c.addObjectUse(object.Name, UseMetadata, object.Name, path, locator.rangeFor(match[2], match[3]), map[string]string{"source": "schema_token"})
	}
}

func (c *schemaUseCollector) collectSOQL(path, source string, locator sourceLocator) {
	for _, literal := range soqlLiterals(source) {
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

func (c *schemaUseCollector) addTriggerObjectUse(trigger typesys.TriggerSymbol) {
	if trigger.ObjectName == "" || trigger.File == "" {
		return
	}
	data, err := os.ReadFile(trigger.File)
	if err != nil {
		return
	}
	source := string(data)
	locator := newSourceLocator(source)
	re := regexp.MustCompile(`(?i)\bon\s+` + regexp.QuoteMeta(trigger.ObjectName) + `\b`)
	if match := re.FindStringIndex(source); match != nil {
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
	out := make([]schema.Object, 0, len(c.objects))
	for _, object := range c.objects {
		out = append(out, object)
	}
	sort.Slice(out, func(i, j int) bool {
		return len(out[i].Name) > len(out[j].Name)
	})
	return out
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

func soqlLiterals(source string) []soqlLiteral {
	var out []soqlLiteral
	for i := 0; i < len(source); i++ {
		if source[i] != '[' {
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

func followsSchemaFields(source string, offset int) bool {
	rest := strings.TrimLeft(source[offset:], " \t\r\n")
	return strings.HasPrefix(strings.ToLower(rest), ".fields")
}
