package sema

func semaGeneratedHistoryType(name string, model *semaTypeMemberView) bool {
	members, _, ok := semaLookupTypeMembers(model, name)
	return ok && members.sobject && members.generatedHistory
}
