package sema

import "github.com/glade-sh/glade/internal/apexversion"

func forLoopScopeCapturedAPI(api string) bool {
	major, ok := apexversion.Major(api)
	return ok && major >= 62 && major <= 68
}

// C194S001-S005 and S063/S066/S069/S071: a later enclosing local
// cannot collide with an earlier loop binding. The text collector visits
// declaration kinds in separate passes, so collection order is not visibility.
// S006-S010 and S075-S078 retain parent and self-redeclaration rejections.
func (s semaScopeModel) forLoopScopeConflictingLocal(api, key string, start, scopeStart, scopeEnd int, classicFor bool) (semaLocal, bool) {
	captured := forLoopScopeCapturedAPI(api)
	if !captured && classicFor {
		// Source 61 also accepts a classic-for initializer followed by an
		// enclosing local of the same name. Other legacy binding kinds keep
		// their existing boundary until independently observed.
		major, ok := apexversion.Major(api)
		captured = ok && major == 61
	}
	if !captured {
		return s.conflictingLocalKey(key, scopeStart, scopeEnd)
	}
	for _, local := range s.locals {
		if s.localKey(local) != key || local.scopeStart > scopeStart || local.scopeEnd < scopeEnd {
			continue
		}
		if (local.scopeStart == scopeStart && local.scopeEnd == scopeEnd) || local.start <= start {
			return local, true
		}
	}
	return semaLocal{}, false
}
