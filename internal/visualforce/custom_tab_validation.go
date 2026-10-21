package visualforce

import (
	"path/filepath"
	"strings"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/resource"
)

// customTabURLTarget validates the documented custom-tab URLFOR target paired
// with its object type. It does not enable $Action evaluation during rendering.
func (ctx *expressionValidationContext) customTabURLTarget(target, objectType Expression) (bool, error) {
	parts, static := standardStaticFieldPath(target)
	typeParts, staticType := standardStaticFieldPath(objectType)
	if !static || len(parts) != 3 || !strings.EqualFold(parts[0], "$Action") || !strings.EqualFold(parts[2], "Tab") ||
		!staticType || len(typeParts) != 2 || !strings.EqualFold(typeParts[0], "$ObjectType") || !strings.EqualFold(parts[1], typeParts[1]) {
		return false, nil
	}
	object, found := ctx.objects[strings.ToLower(parts[1])]
	if !found && ctx.namespace != "" && strings.Count(parts[1], "__") == 1 {
		object, found = ctx.objects[strings.ToLower(ctx.namespace+"__"+parts[1])]
	}
	if !found || object.CustomSettingsType != "" {
		return false, nil
	}
	for _, path := range ctx.tabFiles {
		name := filepath.Base(path)
		if dot := strings.IndexByte(name, '.'); dot >= 0 {
			name = name[:dot]
		}
		qualified := name
		if ctx.namespace != "" && strings.Count(name, "__") == 1 {
			qualified = ctx.namespace + "__" + name
		}
		if !strings.EqualFold(qualified, object.Name) {
			continue
		}
		// Load only the referenced tab. Unrelated malformed tab metadata must
		// not become a prerequisite for this expression or for page indexing.
		registry, err := resource.LoadProject(project.Project{TabFiles: []string{path}})
		if err != nil {
			return false, err
		}
		for _, tab := range registry.Tabs {
			if tab.Custom && strings.EqualFold(tab.SObjectName, name) {
				return true, nil
			}
		}
	}
	return false, nil
}
