Map<String, Schema.FieldSet> sets = Account.SObjectType.getDescribe().fieldSets.getMap();
Map<String, Schema.FieldSet> additions = new Map<String, Schema.FieldSet>{'gladetest1__Other' => sets.get('gladetest1__CodexC163Capture')};
sets.putAll(additions);
