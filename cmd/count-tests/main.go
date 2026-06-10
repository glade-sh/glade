package main

import (
	"fmt"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func main() {
	root := "/Users/matt/Radical1/Projects/src-nmb-nu"
	p, _ := project.Load(root)
	s, _ := gladeschema.LoadProject(p)
	idx := typesys.Build(p, s)
	fmt.Println(len(apextest.Discover(idx, apextest.Options{})))
}
