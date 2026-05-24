package vm

import (
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
