Map<String, Schema.FieldSet> sets = Account.SObjectType.getDescribe().fieldSets.getMap();
sets.put('samplepkg__Other', sets.get('samplepkg__CodexC163Capture'));
