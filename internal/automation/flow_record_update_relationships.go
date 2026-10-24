package automation

import (
	"strings"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sobject"
	"github.com/glade-sh/glade/internal/storage"
)

// A related-record update can name a triggering-record relationship without
// repeating its target object. Resolve standard relationships from describe
// metadata before lowering the update; relationship names need not be object
// names, and polymorphic relationships do not identify a single target.
func flowResolveStandardRelationshipUpdates(raw *flowXML, definitions ...storage.ObjectDefinition) {
	if len(raw.RecordUpdates) == 0 || !strings.EqualFold(strings.TrimSpace(raw.Start.TriggerType), "RecordAfterSave") {
		return
	}
	definition, ok := storage.StandardObjectDefinition(strings.TrimSpace(raw.Start.Object))
	if len(definitions) > 0 {
		definition, ok = definitions[0], true
	}
	if !ok {
		return
	}
	for i := range raw.RecordUpdates {
		update := &raw.RecordUpdates[i]
		if strings.TrimSpace(update.Object) != "" {
			continue
		}
		reference := strings.TrimSpace(update.InputReference)
		relationship := ""
		for _, prefix := range []string{"$Record.", "Record."} {
			if strings.HasPrefix(reference, prefix) {
				relationship = strings.TrimPrefix(reference, prefix)
				break
			}
		}
		if relationship == "" || strings.Contains(relationship, ".") {
			continue
		}
		for _, field := range definition.Fields {
			if field.Type != storage.FieldReference || len(field.ReferenceTo) != 1 || !strings.EqualFold(storage.ParentRelationshipName(field), relationship) {
				continue
			}
			update.Object = field.ReferenceTo[0]
			break
		}
	}
}

func flowProjectRelationshipDefinitions(p project.Project) map[string]storage.ObjectDefinition {
	if len(p.FlowFiles) == 0 || len(p.ObjectFiles) == 0 && len(p.FieldFiles) == 0 {
		return nil
	}
	s, err := schema.LoadProject(p)
	if err != nil {
		return nil
	}
	definitions := make(map[string]storage.ObjectDefinition)
	for name, describe := range sobject.BuildDescribeRegistry(s).Objects {
		definitions[name] = sobject.ToObjectDefinition(describe)
	}
	return definitions
}
