package typesys

import (
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/packageartifact"
)

func TestArtifactTypeSymbolNestingNamespaceRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, namespace := range []string{"", "meadow"} {
			for _, kind := range []apexast.DeclarationKind{apexast.DeclarationClass, apexast.DeclarationInterface, apexast.DeclarationEnum} {
				for _, tc := range []struct {
					name, local, owner string
					depth              int
				}{
					{"Garden", "Garden", "", 0},
					{"Garden.Leaf", "Leaf", "Garden", 1},
					{"Garden.Leaf.Seed", "Seed", "Garden.Leaf", 2},
				} {
					t.Run(api+"/"+namespace+"/"+string(kind)+"/"+tc.name, func(t *testing.T) {
						sym := typeSymbolFromArtifact("meadow", "1.0", api, packageartifact.ApexType{
							Kind: kind, Name: tc.name, Namespace: namespace,
						})
						if sym.LocalName != tc.local || sym.OwnerName != tc.owner || sym.NestingDepth != tc.depth {
							t.Fatalf("artifact identity local=%q owner=%q depth=%d; want local=%q owner=%q depth=%d",
								sym.LocalName, sym.OwnerName, sym.NestingDepth, tc.local, tc.owner, tc.depth)
						}
						if sym.Name != tc.name || sym.Namespace != "meadow" || sym.EffectiveAPIVersion != api || !sym.Artifact || !sym.Dependency {
							t.Fatalf("artifact metadata changed: %#v", sym)
						}
					})
				}
			}
		}
	}
}
