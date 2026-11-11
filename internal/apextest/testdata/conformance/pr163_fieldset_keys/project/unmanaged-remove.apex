Map<String, Schema.FieldSet> sets = Account.SObjectType.getDescribe().fieldSets.getMap();
sets.remove('CODEXC163CAPTURE');
