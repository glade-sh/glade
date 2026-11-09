package soql

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// Data category SOQL names carry __c; selection records store the bare names.
func knowledgeCategoryName(name string) string {
	if len(name) > 3 && strings.EqualFold(name[len(name)-3:], "__c") {
		return name[:len(name)-3]
	}
	return name
}

func knowledgeCategoryPath(categories []storage.DataCategory, name string, path []string) ([]string, bool) {
	for _, category := range categories {
		next := append(append([]string(nil), path...), category.Name)
		if strings.EqualFold(category.Name, name) {
			return next, true
		}
		if found, ok := knowledgeCategoryPath(category.Children, name, next); ok {
			return found, true
		}
	}
	return nil, false
}

func knowledgeCategoryPathPrefix(prefix, path []string) bool {
	if len(prefix) > len(path) {
		return false
	}
	for i := range prefix {
		if !strings.EqualFold(prefix[i], path[i]) {
			return false
		}
	}
	return true
}

func matchesKnowledgeDataCategory(org storage.OrgState, record storage.Record, query Query) bool {
	groupName := knowledgeCategoryName(query.DataCategoryGroup)
	categoryName := knowledgeCategoryName(query.DataCategoryName)
	var categories []storage.DataCategory
	for _, group := range org.Metadata.DataCategoryGroups {
		if strings.EqualFold(group.Name, groupName) {
			categories = group.Categories
			break
		}
	}
	target, ok := knowledgeCategoryPath(categories, categoryName, nil)
	if !ok {
		return false
	}
	objectName, ok := storage.ResolveObjectName(org, "Knowledge__DataCategorySelection")
	if !ok {
		return false
	}
	for _, selection := range org.Objects[objectName].Records {
		if selection.System.IsDeleted {
			continue
		}
		parent, _ := selection.GetField("ParentId")
		group, _ := selection.GetField("DataCategoryGroupName")
		name, _ := selection.GetField("DataCategoryName")
		if !storage.IDsEqual(idFromValue(parent), record.ID) || !strings.EqualFold(group.String, groupName) {
			continue
		}
		selected, ok := knowledgeCategoryPath(categories, name.String, nil)
		if !ok {
			continue
		}
		switch query.DataCategoryOperator {
		case "AT":
			if strings.EqualFold(name.String, categoryName) {
				return true
			}
		case "ABOVE":
			if knowledgeCategoryPathPrefix(selected, target) {
				return true
			}
		case "BELOW":
			if knowledgeCategoryPathPrefix(target, selected) {
				return true
			}
		case "ABOVE_OR_BELOW":
			if knowledgeCategoryPathPrefix(target, selected) || knowledgeCategoryPathPrefix(selected, target) {
				return true
			}
		}
	}
	return false
}
