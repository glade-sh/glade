package storage

// KnowledgeArticleVersionReadView exposes Lightning Knowledge versions through
// their generic read type, as observed at APIs 62/67. It does not persist rows
// or change the concrete object's write policy.
func KnowledgeArticleVersionReadView(org *OrgState) *OrgState {
	if org == nil {
		return org
	}
	sourceName, known := ResolveObjectName(*org, "Knowledge__kav")
	if !known || len(org.Objects[sourceName].Records) == 0 {
		return org
	}
	const targetName = "KnowledgeArticleVersion"
	target, exists := org.Objects[targetName]
	if !exists {
		definition, known := StandardObjectDefinition(targetName)
		if !known {
			return org
		}
		target.Definition = definition
	}
	records := cloneRecordMap(target.Records)
	if records == nil {
		records = make(map[ID]Record, len(org.Objects[sourceName].Records))
	}
	for id, source := range org.Objects[sourceName].Records {
		record := Record{ID: source.ID, Object: targetName, System: source.System, Fields: make(map[string]Value)}
		for name, value := range source.Fields {
			canonical, known := ResolveFieldName(target.Definition, org.Namespace, name)
			if !known {
				continue
			}
			record.Fields[canonical] = value.Clone()
			if source.HasExplicitNull(name) {
				if record.ExplicitNulls == nil {
					record.ExplicitNulls = make(map[string]bool)
				}
				record.ExplicitNulls[canonical] = true
			}
		}
		records[id] = record
	}
	out := *org
	out.Objects = make(map[string]ObjectState, len(org.Objects)+1)
	for name, object := range org.Objects {
		out.Objects[name] = object
	}
	target.Records, target.Indexes = records, nil
	target.RecordsShared, target.IndexesShared = false, false
	out.Objects[targetName] = target
	if !exists {
		out.objectNameCache = nil
		out.ClearRuntimeSchemaStamp()
	}
	return &out
}
