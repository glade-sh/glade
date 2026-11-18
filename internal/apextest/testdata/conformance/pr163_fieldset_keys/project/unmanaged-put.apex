Map<String, Schema.FieldSet> sets = Account.SObjectType.getDescribe().fieldSets.getMap();
sets.put('Other', sets.get('CodexC163Capture'));
