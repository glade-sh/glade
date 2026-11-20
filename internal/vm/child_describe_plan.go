package vm

import (
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

type childDescribePlan struct {
	all            []string
	always         []string
	byParent       map[string][]string
	physicalByFold map[string]string
	fallback       bool
}

func allDescribeChildNames(org *storage.OrgState) []string {
	names := make([]string, 0, len(org.Objects))
	for name := range org.Objects {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func buildChildDescribePlan(org *storage.OrgState) *childDescribePlan {
	p := &childDescribePlan{all: allDescribeChildNames(org), byParent: map[string][]string{}, physicalByFold: map[string]string{}}
	for _, child := range p.all {
		fold := strings.ToLower(strings.TrimSpace(child))
		// The VM's prepared-definition cache uses this key; preserve collisions
		// and unusual host-controlled spellings through the original scan.
		if _, duplicate := p.physicalByFold[fold]; duplicate || !isASCII(child) || child != strings.TrimSpace(child) || child == "" {
			p.fallback = true
			return p
		}
		p.physicalByFold[fold] = child
		d := org.Objects[child].Definition
		if d.APIName != child || !storage.ChildRelationshipTargetsComplete(d) {
			p.always = append(p.always, child)
			continue
		}
		keys := map[string]bool{}
		ambiguous := false
		for _, edge := range d.Relations {
			for _, target := range edge.ParentObjects {
				if !isASCII(target) {
					ambiguous = true
					break
				}
				for _, key := range jsonRelationshipParentCandidateKeys(target) {
					keys[key] = true
				}
			}
			if ambiguous {
				break
			}
		}
		if ambiguous {
			p.always = append(p.always, child)
			continue
		}
		for key := range keys {
			p.byParent[key] = append(p.byParent[key], child)
		}
	}
	return p
}

func (vm *VM) describeChildCandidateNames(parent string, completeSource bool) []string {
	full := func() []string {
		return allDescribeChildNames(vm.Org)
	}
	if vm.Org.RuntimeSchemaStamp == "" || !isASCII(parent) {
		return full()
	}
	// These relations are synthesized even when a stored definition has
	// no corresponding reference field (User and group/equivalent aliases).
	for _, edge := range syntheticSystemParentRelationsForDefinition(storage.ObjectDefinition{}) {
		if relationshipTargetsObject(edge, parent) {
			return full()
		}
	}
	p := vm.childDescribePlan
	if p == nil {
		if completeSource {
			// Only a known source-backed transition which kept the generation
			// and stamp can share its *completed* shape with another clone.
			p = vm.Org.RuntimeGenerationCacheValue("complete-describe-child-edges", func() any {
				return buildChildDescribePlan(vm.Org)
			}).(*childDescribePlan)
		} else {
			p = buildChildDescribePlan(vm.Org)
		}
		vm.childDescribePlan = p
	}
	if p.fallback || len(p.all) != len(vm.Org.Objects) {
		return full()
	}
	selected := make(map[string]bool, len(p.always))
	for _, child := range p.always {
		selected[child] = true
	}
	for _, key := range jsonRelationshipParentCandidateKeys(parent) {
		for _, child := range p.byParent[key] {
			selected[child] = true
		}
	}
	// Honor any already-published/cached preparation from a different VM
	// read route. It remains the source of truth for the stock builder.
	for key := range vm.describeDefCache {
		if name, ok := p.physicalByFold[key]; ok {
			selected[name] = true
		}
	}
	names := make([]string, 0, len(selected))
	for name := range selected {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
