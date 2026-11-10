package storage

import "github.com/glade-sh/glade/internal/schema"

// The project schema canonicalizes the standard object a scratch
// definition's objectSettings names through the standard catalog.
func init() {
	schema.StandardObjectNameResolver = ResolveKnownStandardObjectName
}
