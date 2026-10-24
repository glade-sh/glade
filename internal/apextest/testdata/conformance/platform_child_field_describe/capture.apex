Map<String,List<String>> want = new Map<String,List<String>>{
 'FinanceTransaction' => new List<String>{'DestinationEntityId','LegalEntityId','ParentReferenceEntityId','ReferenceEntityId','SourceEntityId'},
 'FinanceBalanceSnapshot' => new List<String>{'LegalEntityId','ReferenceEntityId'}};
List<String> rows = new List<String>();
for (String obj : new List<String>{'FinanceTransaction','FinanceBalanceSnapshot'}) {
  Schema.SObjectType t = Schema.getGlobalDescribe().get(obj);
  if (t == null) { rows.add(obj + '@@*@@unavailable'); continue; }
  Map<String,Schema.SObjectField> fm = t.getDescribe().fields.getMap();
  for (String f : want.get(obj)) {
    Schema.SObjectField tok = fm.get(f);
    if (tok == null) { rows.add(obj + '@@' + f + '@@absent'); continue; }
    Schema.DescribeFieldResult d = tok.getDescribe();
    List<String> refs = new List<String>();
    for (Schema.SObjectType r : d.getReferenceTo()) { refs.add(String.valueOf(r)); }
    refs.sort();
    rows.add(obj + '@@' + f + '@@ok@@' + d.getType() + '@@' + String.join(refs, ',') + '@@' + d.isNillable() + '@@' + d.isCreateable() + '@@' + d.isUpdateable() + '@@' + d.isDefaultedOnCreate() + '@@' + d.getRelationshipName() + '@@' + d.isAccessible());
  }
}
for (String r : rows) { System.debug(LoggingLevel.ERROR, 'ROW|' + r); }
