package storage

// EntityDefinition exposes this capability separately from the ordinary
// object describe catalog. Keep unobserved objects outside the overlay.
var standardEntityCustomizable = map[string]bool{
	"Account":          true,
	"ApexClass":        false,
	"Contact":          true,
	"EntityDefinition": false,
	"PricebookEntry":   true,
	"User":             true,
	"UserLogin":        false,
}
