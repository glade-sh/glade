package storage

import (
	"regexp"
	"strings"
)

// Consume quoted literals and whole relationship paths so only a reference's
// root is translated. A related object's fields keep their own API names.
var personAccountFormulaToken = regexp.MustCompile(`"(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*'|[A-Za-z_$][A-Za-z0-9_$]*(?:\s*\.\s*[A-Za-z_$][A-Za-z0-9_$]*)*`)

func personAccountFormula(formula string, references map[string]string) string {
	return personAccountFormulaToken.ReplaceAllStringFunc(formula, func(token string) string {
		end := strings.IndexAny(token, ". \t\r\n")
		if end < 0 {
			end = len(token)
		}
		if mirror, ok := references[strings.ToLower(token[:end])]; ok {
			return mirror + token[end:]
		}
		return token
	})
}

// PersonAccountFieldMapping pairs a Person Account field with the Contact
// field it represents on the person contact.
type PersonAccountFieldMapping struct {
	Account string
	Contact string
}

// PersonAccountStandardFieldMappings is the Account-to-Contact standard field
// map that DML uses to synchronize a Person Account's contact. Mirrored Contact
// formulas resolve the same standard references through Account.
func PersonAccountStandardFieldMappings() []PersonAccountFieldMapping {
	mappings := []PersonAccountFieldMapping{
		{"FirstName", "FirstName"},
		{"LastName", "LastName"},
		{"PersonEmail", "Email"},
		{"PersonHomePhone", "HomePhone"},
		{"PersonMobilePhone", "MobilePhone"},
		{"PersonTitle", "Title"},
		{"PersonDepartment", "Department"},
		{"PersonBirthdate", "Birthdate"},
		{"PersonDoNotCall", "DoNotCall"},
		{"PersonHasOptedOutOfEmail", "HasOptedOutOfEmail"},
		{"PersonHasOptedOutOfFax", "HasOptedOutOfFax"},
		{"PersonEmailBouncedReason", "EmailBouncedReason"},
		{"PersonEmailBouncedDate", "EmailBouncedDate"},
	}
	for _, suffix := range []string{"Street", "City", "State", "StateCode", "PostalCode", "Country", "CountryCode"} {
		mappings = append(mappings,
			PersonAccountFieldMapping{"PersonMailing" + suffix, "Mailing" + suffix},
			PersonAccountFieldMapping{"PersonOther" + suffix, "Other" + suffix},
		)
	}
	return mappings
}

// personAccountCustomFieldName derives an Account field only from a declared
// Contact custom field. It does not establish that the field exists in an org.
func personAccountCustomFieldName(contactField string) (string, bool) {
	if len(contactField) <= 3 || !strings.EqualFold(contactField[len(contactField)-3:], "__c") {
		return "", false
	}
	return contactField[:len(contactField)-3] + "__pc", true
}

func applyPersonAccountCustomFields(org *OrgState) {
	contact := org.Objects["Contact"].Definition
	account := org.Objects["Account"]
	account.Definition = account.Definition.Clone()
	references := make(map[string]string)
	// Standard Contact references map to their Person Account fields when
	// both sides are declared (for example Email -> PersonEmail).
	for _, mapping := range PersonAccountStandardFieldMappings() {
		if strings.EqualFold(mapping.Account, mapping.Contact) {
			continue
		}
		if _, ok := ResolveFieldName(contact, "", mapping.Contact); !ok {
			continue
		}
		if _, ok := ResolveFieldName(account.Definition, "", mapping.Account); ok {
			references[strings.ToLower(mapping.Contact)] = mapping.Account
		}
	}
	for key, field := range contact.Fields {
		name := field.APIName
		if name == "" {
			name = key
		}
		if mirror, ok := personAccountCustomFieldName(name); ok {
			references[strings.ToLower(name)] = mirror
			if len(field.ReferenceTo) > 0 && field.RelationshipName != "" {
				references[strings.ToLower(field.RelationshipName)] = strings.TrimSuffix(mirror, "__pc") + "__pr"
			}
		}
	}
	for key, field := range contact.Fields {
		name := field.APIName
		if name == "" {
			name = key
		}
		mirror, ok := personAccountCustomFieldName(name)
		if !ok {
			continue
		}
		field = cloneField(field)
		field.APIName = mirror
		field.Formula = personAccountFormula(field.Formula, references)
		field.ChildRelationshipName = ""
		// Required stays declared (describe nillable=false). DML enforces it
		// for Person Account records only; see isDMLRequiredFieldForRecord.
		if field.Encrypted {
			field.DisplayType = "ENCRYPTEDSTRING"
		}
		if len(field.ReferenceTo) > 0 {
			field.RelationshipName = strings.TrimSuffix(mirror, "__pc") + "__pr"
			found := false
			for _, relation := range account.Definition.Relations {
				if strings.EqualFold(relation.Field, mirror) && strings.EqualFold(relation.ParentRelationship, field.RelationshipName) {
					found = true
					break
				}
			}
			if !found {
				account.Definition.Relations = append(account.Definition.Relations, Relationship{Field: mirror, ParentObjects: append([]string(nil), field.ReferenceTo...), ParentRelationship: field.RelationshipName})
			}
		}
		account.Definition.Fields[mirror] = field
	}
	org.Objects["Account"] = account
}
