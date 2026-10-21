package sema

import (
	"errors"
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/soql"
)

func semiJoinCompileDiagnosticMessage(ctx queryTextContext, err error) (string, bool) {
	var captured *soql.SemiJoinError
	if errors.As(err, &captured) {
		if captured.CompileMessage != "" {
			return captured.CompileMessage, true
		}
		if captured.CompileObject != "" {
			if offset := semiJoinNestedObjectOffset(ctx.queryText, captured.CompileObject); offset >= 0 {
				return semiJoinNativeDiagnostic(ctx, captured.Message, captured.CompileObject, offset, true).NativeMessage, true
			}
		}
		return captured.Message, true
	}
	return "", false
}

func semiJoinNestedObjectOffset(query, object string) int {
	spans := newSemaCodeSpans(query)
	depth := 0
	for i := 0; i < len(query); i++ {
		if !spans.contains(i) {
			continue
		}
		switch query[i] {
		case '(':
			depth++
		case ')':
			depth--
		}
		if depth != 2 || !wordAtFold(query, i, "FROM") {
			continue
		}
		offset := i + len("FROM")
		for offset < len(query) && isWhitespace(query[offset]) {
			offset++
		}
		end := offset + len(object)
		if end <= len(query) && strings.EqualFold(query[offset:end], object) && (end == len(query) || !isQueryIdentPart(query[end])) {
			return offset
		}
	}
	return -1
}

func (c querySemanticsChecker) semiJoinOperandDiagnostic(objectName string, condition soql.Condition, ctx queryTextContext, cursor int) (diagnostic.Diagnostic, bool) {
	if condition.Subquery == nil {
		return diagnostic.Diagnostic{}, false
	}
	fieldName := soql.SemiJoinOperandField(objectName, condition.Field)
	field, known := c.field(objectName, fieldName)
	prefix, _, dotted := strings.Cut(fieldName, ".")
	_, _, relationship := c.relationshipField(objectName, prefix)
	if !c.hasFieldMetadata(objectName) {
		// Named-class indexes can mark standard objects partial. The complete
		// catalog still decides known standard operands; unknown org-owned
		// fields/relationships keep the existing incomplete-metadata boundary.
		provider := semaStandardSObjectFieldProviderFor(objectName)
		if provider == nil {
			return diagnostic.Diagnostic{}, false
		}
		field, known = provider.lookup(fieldName)
		relationship = false
		if dotted {
			provider.visit(func(candidate schema.Field) {
				if strings.EqualFold(candidate.RelationshipName, prefix) && len(candidate.ReferenceTo) > 0 {
					relationship = true
				}
			})
		}
		if !known && !relationship {
			return diagnostic.Diagnostic{}, false
		}
	}
	reference := len(field.ReferenceTo) > 0 && (strings.EqualFold(field.Type, "Reference") || strings.EqualFold(field.Type, "Lookup") || strings.EqualFold(field.Type, "MasterDetail"))
	rejection := soql.SemiJoinOperandRejection(objectName, condition.Field, known, reference, dotted && relationship)
	if rejection == nil {
		return diagnostic.Diagnostic{}, false
	}
	offset := findQueryIdentifier(ctx.queryText, condition.Field, cursor)
	return semiJoinNativeDiagnostic(ctx, rejection.Message, condition.Field, offset, dotted), true
}

// The captured compiler excerpts differ from the existing compact query
// diagnostics. Keep their native projection separate from source ranges and
// dynamic QueryException text. Captures cover zero or one preceding simple bind.
func semiJoinNativeDiagnostic(ctx queryTextContext, message, token string, offset int, excerpt bool) diagnostic.Diagnostic {
	item := ctx.diagnostic("GLADESEMA_QUERY_CONTRACT", message, token, offset)
	item.NativeMessage = message
	if !excerpt || offset < 0 {
		return item
	}
	column := offset + 3
	binds := semaInlineBindPattern.FindAllStringSubmatchIndex(maskInlineQueryComments(ctx.queryText[:offset]), -1)
	if len(binds) > 1 {
		return ctx.nativeSOQLDiagnostic("GLADESEMA_QUERY_CONTRACT", message, token, offset, true)
	}
	if len(binds) == 1 {
		// The single preceding bind occupies eight columns in these native excerpts.
		column += 8 - (binds[0][1] - binds[0][0])
	}
	start := max(0, column-31)
	for start > 0 && !isWhitespace(ctx.queryText[start-1]) {
		start--
	}
	end := min(len(ctx.queryText), column+30)
	for end < len(ctx.queryText) && !isWhitespace(ctx.queryText[end]) {
		end++
	}
	leading := ""
	if ctx.file == "" {
		leading = "\n"
	}
	item.NativeMessage = fmt.Sprintf("%s%s\n%s^\nERROR at Row:1:Column:%d\n%s", leading, ctx.queryText[start:end], strings.Repeat(" ", max(0, column-1-start)), column, message)
	return item
}
