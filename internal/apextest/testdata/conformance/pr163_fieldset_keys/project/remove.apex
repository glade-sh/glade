Map<String, Schema.FieldSet> sets = Account.SObjectType.getDescribe().fieldSets.getMap();
sets.remove('SAMPLEPKG__CODEXC163CAPTURE');
