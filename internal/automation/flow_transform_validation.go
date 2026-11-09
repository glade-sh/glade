package automation

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
	"strings"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sobject"
	"github.com/glade-sh/glade/internal/storage"
)

// A schema overlay does not prove a field absent. Only an explicitly complete
// snapshot permits rejecting an unknown field; duplicate output targets need
// no schema. Both controls were rejected by native Flow deployment at 62/67.
func flowValidateProjectTransformTargets(p project.Project, idx *Index) {
	org := storage.NewOrgState()
	org.Namespace = p.Namespace
	complete := make(map[string]bool)
	loaded := false
	loadSchema := func() {
		if loaded {
			return
		}
		loaded = true
		if p.SchemaSnapshot != "" {
			if s, err := schema.LoadProject(p); err == nil {
				for _, object := range s.Objects {
					if object.CompleteSnapshot {
						complete[strings.ToLower(object.Name)] = true
					}
				}
				for name, describe := range sobject.BuildDescribeRegistry(s).Objects {
					org.Objects[name] = storage.ObjectState{Definition: sobject.ToObjectDefinition(describe)}
				}
			}
		}
	}
	for i := range idx.Flows {
		flow := &idx.Flows[i]
		data, err := os.ReadFile(flow.File)
		if err != nil || !bytes.Contains(data, []byte("transformValues")) {
			continue
		}
		var raw flowXML
		if xml.Unmarshal(data, &raw) != nil || !flowActive(raw.Status) || !strings.EqualFold(strings.TrimSpace(raw.ProcessType), "AutoLaunchedFlow") {
			continue
		}
		for _, transform := range raw.Transforms {
			if !strings.EqualFold(strings.TrimSpace(transform.DataType), "SObject") || strings.EqualFold(strings.TrimSpace(transform.IsCollection), "true") || len(transform.TransformValues) != 1 {
				continue
			}
			loadSchema()
			objectName, found := storage.ResolveObjectName(org, transform.ObjectType)
			definition := org.Objects[objectName].Definition
			canRejectMissing := found && complete[strings.ToLower(objectName)]
			seen := make(map[string]bool)
			for _, action := range transform.TransformValues[0].Actions {
				field := strings.TrimSpace(action.OutputFieldAPIName)
				if field == "" || !strings.EqualFold(strings.TrimSpace(action.TransformType), "Map") {
					continue
				}
				key := strings.ToLower(field)
				_, fieldKnown := storage.ResolveFieldName(definition, org.Namespace, field)
				if seen[key] || (canRejectMissing && !fieldKnown && !flowRefsStandardMemberGap(definition, field)) {
					idx.Diagnostics = append(idx.Diagnostics, diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADEAUTO004", File: flow.File, Message: fmt.Sprintf("flow transform %q has an invalid output field %q", transform.Name, field)})
					flow.Rules = nil
					break
				}
				seen[key] = true
			}
		}
	}
}
