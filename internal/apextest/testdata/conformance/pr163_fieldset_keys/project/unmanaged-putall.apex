Map<String, Schema.FieldSet> sets = Account.SObjectType.getDescribe().fieldSets.getMap();
Map<String, Schema.FieldSet> additions = new Map<String, Schema.FieldSet>{'Other' => sets.get('CodexC163Capture')};
sets.putAll(additions);
