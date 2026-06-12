package lwc

import (
	"regexp"
	"strings"
)

var apiFieldRE = regexp.MustCompile(`(?m)@api\s+([A-Za-z_$][\w$]*)\s*=\s*([^;]+);`)

func ParseAPIProperties(source string) (map[string]string, error) {
	props := map[string]string{}
	for _, match := range apiFieldRE.FindAllStringSubmatch(source, -1) {
		props[match[1]] = strings.TrimSpace(match[2])
	}
	return props, nil
}
