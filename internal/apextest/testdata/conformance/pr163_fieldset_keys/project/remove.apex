Map<String, Schema.FieldSet> sets = Account.SObjectType.getDescribe().fieldSets.getMap();
sets.remove('GLADETEST1__CODEXC163CAPTURE');
