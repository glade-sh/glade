Map<String, Schema.FieldSet> sets = Account.SObjectType.getDescribe().fieldSets.getMap();
Map<String, Schema.FieldSet> additions = new Map<String, Schema.FieldSet>{'samplepkg__Other' => sets.get('samplepkg__CodexC163Capture')};
sets.putAll(additions);
