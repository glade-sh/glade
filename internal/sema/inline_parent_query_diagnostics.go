package sema

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/soql"
)

var capturedContactReturningPath = regexp.MustCompile(`(?i)^FIND '[^'\\]*' IN ALL FIELDS RETURNING Contact\(Id, ([A-Za-z_][A-Za-z0-9_]*)\.([A-Za-z_][A-Za-z0-9_]*)\)$`)
var capturedAccountChildQuery = regexp.MustCompile(`(?i)^SELECT Id,\(SELECT Id FROM ([A-Za-z_][A-Za-z0-9_]*)\) FROM Account LIMIT 1$`)

// The captured parent-path rejections name the missing leaf or relationship,
// rather than the entire dotted path on Contact. Standard field providers are
// authoritative here even when source inference marks Account partial.
func (c querySemanticsChecker) capturedParentReturningDiagnostic(ctx queryTextContext) (diagnostic.Diagnostic, bool) {
	if c.namespace != "" || c.apiVersion < 62 || c.apiVersion > 68 {
		return diagnostic.Diagnostic{}, false
	}
	match := capturedContactReturningPath.FindStringSubmatch(ctx.queryText)
	if match == nil {
		return diagnostic.Diagnostic{}, false
	}
	parent, leaf := match[1], match[2]
	path := parent + "." + leaf
	offset := strings.LastIndex(ctx.queryText, path)
	if strings.EqualFold(parent, "Account") {
		if _, target, known := c.relationshipField("Contact", parent); !known || !strings.EqualFold(target, "Account") {
			return diagnostic.Diagnostic{}, false
		}
		var known bool
		if strings.Contains(leaf, "__") {
			// Double-underscore spellings are outside the captured bare leaf;
			// retain their existing project lookup, including unknown suffixes.
			_, known = c.field("Account", leaf)
		} else {
			// Named-source inference can add the queried leaf to partial Account
			// metadata. Query text cannot establish a standard field's existence.
			provider := semaStandardSObjectFieldProviderFor("Account")
			if provider == nil {
				return diagnostic.Diagnostic{}, false
			}
			_, known = provider.lookup(leaf)
		}
		if known {
			return diagnostic.Diagnostic{}, false
		}
		message := soql.MissingColumnMessage(leaf, "Account")
		return ctx.capturedQueryDiagnostic("GLADESEMA_QUERY_FIELD", message, path, offset), true
	}
	if !strings.EqualFold(leaf, "Name") {
		return diagnostic.Diagnostic{}, false
	}
	if _, _, known := c.relationshipField("Contact", parent); known {
		return diagnostic.Diagnostic{}, false
	}
	if _, field := c.field("Contact", parent); field {
		return diagnostic.Diagnostic{}, false
	}
	message := soql.MissingRelationshipMessage(parent, "field path")
	return ctx.capturedQueryDiagnostic("GLADESEMA_QUERY_RELATIONSHIP", message, path, offset), true
}

func (c querySemanticsChecker) capturedAccountChildDiagnostic(ctx queryTextContext) (diagnostic.Diagnostic, bool) {
	if c.namespace != "" || c.apiVersion < 62 || c.apiVersion > 68 {
		return diagnostic.Diagnostic{}, false
	}
	match := capturedAccountChildQuery.FindStringSubmatch(ctx.queryText)
	if match == nil {
		return diagnostic.Diagnostic{}, false
	}
	relationship := match[1]
	if _, known := c.childObjectForRelationship("Account", relationship); known {
		return diagnostic.Diagnostic{}, false
	}
	offset := strings.Index(ctx.queryText, relationship)
	message := soql.MissingRelationshipMessage(relationship, "FROM part of query call")
	return ctx.capturedQueryDiagnostic("GLADESEMA_QUERY_RELATIONSHIP", message, relationship, offset), true
}

// These inline captures retain newlines and a route-specific leading newline.
// Preserve the existing diagnostic message for other consumers; attach native
// presentation only to the measured query shapes above.
func (ctx queryTextContext) capturedQueryDiagnostic(code, message, token string, offset int) diagnostic.Diagnostic {
	item := ctx.nativeSOQLDiagnostic(code, message, token, offset, true)
	column := offset + 2
	sosl := startsWithQueryKeyword(ctx.queryText, "FIND")
	if sosl {
		column = offset + 1
	}
	start := max(0, column-30)
	for start > 0 && !isWhitespace(ctx.queryText[start-1]) {
		start--
	}
	caret := offset + 1 - start
	if sosl {
		// This captured Contact RETURNING form starts its excerpt at ALL.
		start = strings.LastIndex(strings.ToUpper(ctx.queryText), " IN ALL FIELDS RETURNING") + len(" IN ")
		caret = offset - start
	}
	end := min(len(ctx.queryText), column+30)
	for end < len(ctx.queryText) && !isWhitespace(ctx.queryText[end]) {
		end++
	}
	prefix := ""
	if ctx.file == "" {
		prefix = "\n"
	}
	item.NativeMessage = prefix + ctx.queryText[start:end] + "\n" + strings.Repeat(" ", max(0, caret)) +
		"^\nERROR at Row:1:Column:" + fmt.Sprint(column) + "\n" + message
	return item
}
