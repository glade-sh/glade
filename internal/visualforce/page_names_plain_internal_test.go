package visualforce

// onlyPlain reports whether names is exactly the page that does not use the
// rejected controller. A rejected controller leaves its own page out; a sibling
// page that loads on its own stays registered.
func onlyPlain(names []string) bool {
	return len(names) == 1 && names[0] == "Plain"
}
