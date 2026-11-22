package testdaemon

import (
	"path/filepath"
	"reflect"
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/watch"
)

// bodyEditKeepsDeclarations accepts only a single existing local class whose
// declarations and metadata are unchanged apart from source positions.
func bodyEditKeepsDeclarations(previous, next typesys.Index, oldArtifacts, newArtifacts typesys.BuildArtifacts, changes []watch.Change) bool {
	if len(changes) != 1 || changes[0].Kind != watch.FileKindApexClass || changes[0].Op != watch.ChangeModified || len(previous.Diagnostics) != 0 || len(next.Diagnostics) != 0 || !sameOtherIndexFacts(previous, next) {
		return false
	}
	path := filepath.Clean(changes[0].Path)
	var before, after, oldTypes, newTypes []typesys.TypeSymbol
	names := make(map[string]int)
	for _, typ := range previous.Types {
		if !typ.Dependency && !typ.Artifact {
			names[strings.ToLower(typ.Name)]++
		}
		if filepath.Clean(typ.File) == path {
			if typ.Dependency || typ.Artifact || !typ.HasSourceSnapshot() {
				return false
			}
			oldTypes = append(oldTypes, typ)
			before = append(before, declarationWithoutPositions(typ))
		}
	}
	for _, typ := range next.Types {
		if filepath.Clean(typ.File) == path {
			if typ.Dependency || typ.Artifact || !typ.HasSourceSnapshot() {
				return false
			}
			newTypes = append(newTypes, typ)
			after = append(after, declarationWithoutPositions(typ))
		}
	}
	if len(before) == 0 || len(before) != len(after) {
		return false
	}
	for _, typ := range before {
		if names[strings.ToLower(typ.Name)] != 1 {
			return false
		}
	}
	return reflect.DeepEqual(before, after) && changeWithinOneMethod(oldTypes, newTypes, oldArtifacts, newArtifacts)
}

func sameOtherIndexFacts(previous, next typesys.Index) bool {
	left, right := reflect.ValueOf(previous), reflect.ValueOf(next)
	typ := left.Type()
	for i := 0; i < left.NumField(); i++ {
		field := typ.Field(i)
		if field.PkgPath != "" || field.Name == "Types" {
			continue
		}
		if !reflect.DeepEqual(left.Field(i).Interface(), right.Field(i).Interface()) {
			return false
		}
	}
	return true
}

func declarationWithoutPositions(typ typesys.TypeSymbol) typesys.TypeSymbol {
	typ.Range = diagnostic.Range{}
	typ.Annotations = annotationsWithoutPositions(typ.Annotations)
	typ.Members = append([]typesys.MemberSymbol(nil), typ.Members...)
	for i := range typ.Members {
		member := &typ.Members[i]
		member.Range = diagnostic.Range{}
		member.BodyRange = nil
		member.Annotations = annotationsWithoutPositions(member.Annotations)
		member.Parameters = append([]apexast.Parameter(nil), member.Parameters...)
		for j := range member.Parameters {
			member.Parameters[j].Range = diagnostic.Range{}
			member.Parameters[j].Annotations = annotationsWithoutPositions(member.Parameters[j].Annotations)
		}
		member.Accessors = append([]apexast.Accessor(nil), member.Accessors...)
		for j := range member.Accessors {
			member.Accessors[j].Range = diagnostic.Range{}
			member.Accessors[j].BodyRange = nil
			member.Accessors[j].Annotations = annotationsWithoutPositions(member.Accessors[j].Annotations)
		}
	}
	return typ
}

func annotationsWithoutPositions(annotations []apexast.Annotation) []apexast.Annotation {
	result := append([]apexast.Annotation(nil), annotations...)
	for i := range result {
		result[i].Range = diagnostic.Range{}
		result[i].Arguments = append([]apexast.AnnotationArgument(nil), result[i].Arguments...)
		for j := range result[i].Arguments {
			result[i].Arguments[j].Range = diagnostic.Range{}
		}
	}
	return result
}

func changeWithinOneMethod(before, after []typesys.TypeSymbol, oldArtifacts, newArtifacts typesys.BuildArtifacts) bool {
	oldSource, ok := oldArtifacts.SourceForType(before[0])
	if !ok {
		return false
	}
	newSource, ok := newArtifacts.SourceForType(after[0])
	if !ok {
		return false
	}
	oldText, newText := oldSource.NormalizedString(), newSource.NormalizedString()
	for i := range before {
		oldOccurrence, oldOK := oldArtifacts.SourceForType(before[i])
		newOccurrence, newOK := newArtifacts.SourceForType(after[i])
		oldMetadata, oldMetaOK := oldArtifacts.ApexMetadataForType(before[i])
		newMetadata, newMetaOK := newArtifacts.ApexMetadataForType(after[i])
		if !oldOK || !newOK || !oldMetaOK || !newMetaOK || oldOccurrence.NormalizedString() != oldText || newOccurrence.NormalizedString() != newText || oldMetadata != newMetadata {
			return false
		}
	}
	if oldText == newText {
		return false
	}
	start := 0
	for start < len(oldText) && start < len(newText) && oldText[start] == newText[start] {
		start++
	}
	oldEnd, newEnd := len(oldText), len(newText)
	for oldEnd > start && newEnd > start && oldText[oldEnd-1] == newText[newEnd-1] {
		oldEnd--
		newEnd--
	}
	for i := range before {
		if len(before[i].Members) != len(after[i].Members) {
			return false
		}
		for j, member := range before[i].Members {
			newer := after[i].Members[j]
			if member.Kind != apexast.DeclarationMethod || member.BodyRange == nil || newer.BodyRange == nil {
				continue
			}
			oldRange, newRange := member.BodyRange, newer.BodyRange
			if oldRange.Start.Offset < start && oldEnd < oldRange.End.Offset && newRange.Start.Offset < start && newEnd < newRange.End.Offset {
				return true
			}
		}
	}
	return false
}
