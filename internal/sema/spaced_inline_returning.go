package sema

import (
	"regexp"
	"strings"

	"github.com/glade-sh/glade/internal/diagnostic"
)

var capturedSpacedInlineReturning = regexp.MustCompile(`(?i)^FIND '[^'\\]*' IN ALL FIELDS RETURNING Contact\(Id, Account \. Name\)$`)

// The spaced-dot twin is rejected by native inline Apex syntax. Dynamic SOSL,
// contiguous paths and other RETURNING spellings retain their existing checks.
func (c querySemanticsChecker) capturedSpacedReturningDiagnostic(ctx queryTextContext) (diagnostic.Diagnostic, bool) {
	if c.namespace != "" || c.apiVersion < 62 || c.apiVersion > 68 || !capturedSpacedInlineReturning.MatchString(ctx.queryText) {
		return diagnostic.Diagnostic{}, false
	}
	offset := strings.Index(ctx.queryText, ".")
	return ctx.diagnostic("GLADESEMA_SOSL_PARSE", "Unexpected token '.'.", ".", offset), true
}
