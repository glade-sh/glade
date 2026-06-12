package aura

import (
	"fmt"
	"regexp"
)

var (
	outAppExtendsRE  = regexp.MustCompile(`(?i)\bextends\s*=\s*["']ltng:outApp["']`)
	dependencyAttrRE = regexp.MustCompile(`(?i)<aura:dependency[^>]*\bresource\s*=\s*["']([^"']+)["']`)
)

type OutApp struct {
	Name         string
	Extends      string
	Dependencies []string // e.g. c:myWidget
}

func ParseOutApp(name, source string) (OutApp, error) {
	app := OutApp{Name: name}
	if !outAppExtendsRE.MatchString(source) {
		return OutApp{}, fmt.Errorf("%q is not a Lightning Out app", name)
	}
	app.Extends = "ltng:outApp"
	for _, m := range dependencyAttrRE.FindAllStringSubmatch(source, -1) {
		app.Dependencies = append(app.Dependencies, m[1])
	}
	return app, nil
}
