package dml

import "github.com/glade-sh/glade/internal/storage"

func (p *formulaParser) customPermissionGranted(name string) bool {
	userID := p.userID
	fields := p.permissionUserFields
	if userID == "" && p.org != nil {
		userID, fields = p.org.FormulaContext()
	}
	if userID == "" {
		userID = storage.DefaultUserID
	}
	return storage.CustomPermissionGranted(p.org, userID, fields, name)
}
