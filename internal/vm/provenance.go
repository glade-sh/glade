package vm

import (
	"path/filepath"
	"strconv"
	"strings"
)

type symbolOrigin string

const (
	symbolOriginProject    symbolOrigin = "project"
	symbolOriginDependency symbolOrigin = "dependency"
)

func symbolOriginFromDependency(dependency bool) symbolOrigin {
	if dependency {
		return symbolOriginDependency
	}
	return symbolOriginProject
}

func dependencyPreferenceRank(origin symbolOrigin, preferDependency bool) int {
	if preferDependency == (origin == symbolOriginDependency) {
		return 0
	}
	return 1
}

func fieldOrigin(field Field) symbolOrigin {
	return symbolOriginFromDependency(field.Dependency)
}

func methodOrigin(method Method) symbolOrigin {
	return symbolOriginFromDependency(method.Dependency)
}

func registeredMethodCandidateKey(method Method) string {
	return strings.Join([]string{
		method.ClassName,
		method.Name,
		methodSignature(method),
		strconv.FormatBool(method.IsStatic),
		string(methodOrigin(method)),
		method.File,
	}, "\x00")
}

func registeredMethodSourceAliasKey(method Method) string {
	owner := strings.TrimSpace(method.ClassName)
	methodOwner := strings.TrimSpace(classNameFromMethod(method.Name))
	if owner == "" {
		owner = methodOwner
	}
	if methodOwner != "" && strings.HasSuffix(strings.ToLower(owner), "."+strings.ToLower(methodOwner)) {
		owner = methodOwner
	}
	file := ""
	if strings.TrimSpace(method.File) != "" {
		file = filepath.Clean(method.File)
		if file == "." {
			file = ""
		}
	}
	return strings.Join([]string{
		owner,
		apexMethodMemberName(method.Name),
		methodParamSignature(method),
		strconv.FormatBool(method.IsStatic),
		string(methodOrigin(method)),
		file,
		strconv.Itoa(method.Line),
		strconv.Itoa(method.Column),
	}, "\x00")
}
