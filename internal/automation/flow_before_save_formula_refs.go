package automation

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

var flowRecordMergeReference = regexp.MustCompile(`\{!\s*(\$Record\.[A-Za-z0-9_.]+)\s*\}`)

// flowBeforeSaveFormulaFieldReference reports a before-save flow that reads a
// formula field through $Record. Native deploys reject it (org capture
// flow_assign_routes rejections, API 62.0 and 67.0): a Decision operand, an
// Assignment value, a formula resource and Custom Error text naming
// $Record.<formula>, $Record.<cross-object formula> or
// $Record.<lookup>.<formula> all fail; an entry filter on the field deploys.
func flowBeforeSaveFormulaFieldReference(raw flowXML, definitions map[string]storage.ObjectDefinition) string {
	if !strings.EqualFold(strings.TrimSpace(raw.Start.TriggerType), "RecordBeforeSave") || len(definitions) == 0 {
		return ""
	}
	root, ok := flowDefinitionNamed(definitions, raw.Start.Object)
	if !ok {
		return ""
	}
	var references []string
	add := func(reference string) {
		if reference = strings.TrimSpace(reference); reference != "" {
			references = append(references, reference)
		}
	}
	addText := func(text string) {
		for _, match := range flowRecordMergeReference.FindAllStringSubmatch(text, -1) {
			add(match[1])
		}
	}
	for _, decision := range raw.Decisions {
		for _, rule := range decision.Rules {
			for _, condition := range rule.Conditions {
				add(condition.LeftValueReference)
				add(condition.RightValue.ElementReference)
			}
		}
	}
	for _, assignment := range raw.Assignments {
		for _, item := range assignment.Items {
			add(item.Value.ElementReference)
		}
	}
	for _, update := range raw.RecordUpdates {
		for _, field := range update.Fields {
			add(field.Value.ElementReference)
		}
	}
	for _, formula := range raw.Formulas {
		addText(formula.Expression)
	}
	for _, template := range raw.TextTemplates {
		addText(template.Text)
	}
	for _, customError := range raw.CustomErrors {
		for _, message := range customError.Messages {
			addText(firstNonBlank(message.ErrorMessage, message.Message))
		}
	}
	for _, reference := range references {
		path, ok := strings.CutPrefix(reference, "$Record.")
		if !ok {
			continue
		}
		if flowPathEndsAtFormulaField(root, strings.Split(path, "."), definitions) {
			return fmt.Sprintf(`field integrity exception: unknown (When the TriggerType field is set to "RecordBeforeSave", the %s field isn't supported.)`, reference)
		}
	}
	return ""
}

func flowDefinitionNamed(definitions map[string]storage.ObjectDefinition, name string) (storage.ObjectDefinition, bool) {
	name = strings.TrimSpace(name)
	if definition, ok := definitions[name]; ok {
		return definition, true
	}
	for key, definition := range definitions {
		if strings.EqualFold(key, name) {
			return definition, true
		}
	}
	return storage.ObjectDefinition{}, false
}

func flowPathEndsAtFormulaField(definition storage.ObjectDefinition, parts []string, definitions map[string]storage.ObjectDefinition) bool {
	for len(parts) > 1 {
		next, found := storage.ObjectDefinition{}, false
		for _, field := range definition.Fields {
			if field.Type != storage.FieldReference || len(field.ReferenceTo) != 1 || !strings.EqualFold(storage.ParentRelationshipName(field), parts[0]) {
				continue
			}
			next, found = flowDefinitionNamed(definitions, field.ReferenceTo[0])
			break
		}
		if !found {
			return false
		}
		definition, parts = next, parts[1:]
	}
	name, ok := storage.ResolveFieldName(definition, "", parts[0])
	if !ok {
		return false
	}
	field := definition.Fields[name]
	return field.Type == storage.FieldCalculated && strings.TrimSpace(field.Formula) != ""
}
