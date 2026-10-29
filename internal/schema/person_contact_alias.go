package schema

import "strings"

// Person account custom fields are declared on Contact in source metadata.
// Salesforce exposes the same field metadata on Account with the __pc suffix.
func addPersonContactAliases(objects map[string]*Object, features []string) {
	enabled := false
	for _, feature := range features {
		if strings.EqualFold(feature, "PersonAccounts") {
			enabled = true
			break
		}
	}
	if !enabled {
		return
	}
	contact := objects["Contact"]
	if contact == nil {
		return
	}
	for _, field := range contact.Fields {
		if !strings.HasSuffix(strings.ToLower(field.Name), "__c") {
			continue
		}
		// Requiredness on a Contact cannot become an unconditional requirement
		// for business Accounts. Required and relationship aliases need their
		// own contextual model before they can be projected.
		if field.Required || len(field.ReferenceTo) != 0 {
			continue
		}
		alias := field
		alias.Name = field.Name[:len(field.Name)-3] + "__pc"
		account := objects["Account"]
		if account == nil {
			account = &Object{Name: "Account", Partial: true}
			objects["Account"] = account
		}
		if fieldIndex(account.Fields, alias.Name) < 0 {
			account.Fields = append(account.Fields, alias)
		}
	}
}
